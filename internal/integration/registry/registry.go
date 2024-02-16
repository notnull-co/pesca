package registry

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/notnull-co/dynaclient/pkg/client"
	"github.com/notnull-co/pesca/internal/domain"
	"github.com/rs/zerolog/log"
)

const (
	apiVersion         = "/v2/"
	LIST_TAGS_ENDPOINT = "/tags/list"
	MANIFEST_ENDPOINT  = "/manifests/"

	headerV1 = "application/vnd.docker.distribution.manifest.v1+prettyjws"
	headerV2 = "application/vnd.docker.distribution.manifest.v2+json"
)

type RegistryClient interface {
	PollingImage(registryURL, repositoryName string, strategy domain.PullingStrategy) (domain.Image, error)
}

type registryClient struct {
	*RoundTripper
	strategyFunction map[domain.PullingStrategy]func(tags *tag, registryURL string, repositoryName string) (*domain.Image, error)
}

func NewRegistry() RegistryClient {
	return &registryClient{
		RoundTripper:     newRoundTripper(map[string]string{}),
		strategyFunction: map[domain.PullingStrategy]func(tags *tag, registryURL string, repositoryName string) (*domain.Image, error){},
	}
}

var cachedToken = map[string]struct {
	token     string
	expiresAt time.Time
}{}

func (r *registryClient) setCachedToken(registryURL, repositoryName string) error {
	token, ok := cachedToken[registryURL+repositoryName]
	if !ok || time.Until(token.expiresAt) < 30*time.Second {
		token, err := r.setToken(registryURL, repositoryName)
		if err != nil {
			return err
		}

		cachedToken[registryURL+repositoryName] = struct {
			token     string
			expiresAt time.Time
		}{
			token:     token.Token,
			expiresAt: time.Now().Add(time.Second * time.Duration(token.ExpiresIn)),
		}
	}

	r.AddHeader("Authorization", "Bearer "+cachedToken[registryURL+repositoryName].token)

	return nil
}

func (r *registryClient) setStrategyFunctions() {
	r.strategyFunction[domain.Lexicographic] = r.applyLexicographicStrategy
	r.strategyFunction[domain.LastDate] = r.applyLastDateStrategy
}

func (r *registryClient) setToken(registryURL, repositoryName string) (token, error) {
	c := client.New[any]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion, nil)
	if err != nil {
		return token{}, err
	}

	_, httpResponse, err := c.Do(req)
	if err != nil {
		return token{}, err
	}

	if httpResponse != nil {
		if httpResponse.StatusCode == http.StatusUnauthorized {
			header := httpResponse.Header.Get("WWW-Authenticate")

			realm, service := getAuthenticationParams(header)

			req, err := client.NewRequest(http.MethodGet, realm+"?service="+service+"&scope=repository:"+repositoryName+":pull", nil)
			if err != nil {
				return token{}, err
			}

			newClient := client.New[token]()

			response, httpResponse, err := newClient.Do(req)
			if err != nil {
				return token{}, err
			}

			if httpResponse != nil && httpResponse.StatusCode > http.StatusOK {
				var httpError httpError

				if err := json.Unmarshal(httpResponse.Body(), &httpError); err != nil {
					return token{}, err
				}

				return token{}, &httpError
			}

			return *response, nil
		}
	}

	return token{}, nil
}

func (r *registryClient) listTags(registryURL, repositoryName string) (*tag, error) {
	err := r.setCachedToken(registryURL, repositoryName)
	if err != nil {
		return nil, err
	}

	c := client.New[tag]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion+repositoryName+LIST_TAGS_ENDPOINT, nil)
	if err != nil {
		return nil, err
	}

	c.Transport = r.RoundTripper

	tags, httpResponse, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if httpResponse != nil {
		if httpResponse.StatusCode > http.StatusOK {
			var httpError httpError

			if err := json.Unmarshal(httpResponse.Body(), &httpError); err != nil {
				return nil, err
			}

			return nil, &httpError
		}
	}

	return tags, nil
}

func (r *registryClient) getLastManifestForTagv1(registryURL, repositoryName, tag string) (*domain.Image, error) {
	c := client.New[manifestv1]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion+repositoryName+MANIFEST_ENDPOINT+tag, nil)
	if err != nil {
		return nil, err
	}

	r.AddHeader("Accept", headerV1)

	c.Transport = r.RoundTripper

	response, httpResponse, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if httpResponse != nil {
		if httpResponse.StatusCode > http.StatusOK {
			var httpError httpError

			if err := json.Unmarshal(httpResponse.Body(), &httpError); err != nil {
				return nil, err
			}

			return nil, &httpError
		}
	}

	var lastDate time.Time
	for _, history := range response.History {
		var compatibility v1Compatibility

		if err := json.Unmarshal([]byte(history.V1Compatibility), &compatibility); err != nil {
			return nil, err
		}

		manifestDate, err := time.Parse(time.RFC3339Nano, compatibility.Created)
		if err != nil {
			return nil, err
		}

		diff := manifestDate.Sub(lastDate)

		if diff > 0 {
			lastDate = manifestDate
		}
	}

	hash, err := r.getImageHash(registryURL, repositoryName, tag)
	if err != nil {
		return nil, err
	}

	return &domain.Image{
		Tag:       tag,
		Digest:    hash,
		CreatedAt: lastDate,
	}, nil
}

func (r *registryClient) getImageHash(registryURL, repositoryName, tag string) (string, error) {
	c := client.New[any]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion+repositoryName+MANIFEST_ENDPOINT+tag, nil)
	if err != nil {
		return "", err
	}

	r.AddHeader("Accept", headerV2)

	c.Transport = r.RoundTripper

	_, httpResponse, err := c.Do(req)
	if err != nil {
		return "", err
	}

	if httpResponse != nil {
		if httpResponse.StatusCode > http.StatusOK {
			var httpError httpError

			if err := json.Unmarshal(httpResponse.Body(), &httpError); err != nil {
				return "", err
			}

			return "", &httpError
		}
	}

	shaDigest := httpResponse.Header.Get("Docker-Content-Digest")

	return shaDigest, nil
}

func getAuthenticationParams(realmHeader string) (realm string, service string) {
	realm = strings.Split(strings.Split(realmHeader, "realm=\"")[1], "\"")[0]
	service = strings.Split(strings.Split(realmHeader, "service=\"")[1], "\"")[0]
	return realm, service
}

func (r *registryClient) PollingImage(registryURL, repositoryName string, strategy domain.PullingStrategy) (domain.Image, error) {
	r.setStrategyFunctions()

	tags, err := r.listTags(registryURL, repositoryName)
	if err != nil {
		return domain.Image{}, err
	}

	image, err := r.strategyFunction[strategy](tags, registryURL, repositoryName)
	if err != nil {
		return domain.Image{}, nil
	}

	return *image, nil
}

func (r *registryClient) applyLastDateStrategy(tags *tag, registryURL, repositoryName string) (*domain.Image, error) {
	results := make(chan *domain.Image, len(tags.Tags))

	for _, tag := range tags.Tags {
		go func(tag string) {
			manifest, err := r.getLastManifestForTagv1(registryURL, repositoryName, tag)
			if err != nil {
				log.Fatal().Err(err).Msg("getting last manifest call failed")
				return
			}
			results <- manifest
		}(tag)
	}

	lastImageCreated := getTheLastImageByDateFromTheChannel(results, len(tags.Tags))

	return &lastImageCreated, nil
}

func (r *registryClient) applyLexicographicStrategy(tags *tag, registryURL, repositoryName string) (*domain.Image, error) {
	sort.Strings(tags.Tags)

	lastTagReleased := tags.Tags[len(tags.Tags)-1]

	image, err := r.getLastManifestForTagv1(registryURL, repositoryName, lastTagReleased)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func getTheLastImageByDateFromTheChannel(channel chan *domain.Image, lenTags int) domain.Image {
	var lastImageCreated domain.Image

	for range lenTags {
		manifest := <-channel

		diff := manifest.CreatedAt.Sub(lastImageCreated.CreatedAt)

		if diff > 0 {
			lastImageCreated = *manifest
		}
	}

	return lastImageCreated
}
