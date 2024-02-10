package registry

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
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
	PullingImage(registryURL, repositoryName string) (domain.ManifestTag, error)
}

type registryClient struct {
	*RoundTripper
}

func NewRegistry() RegistryClient {
	return &registryClient{
		RoundTripper: newRoundTripper(map[string]string{}),
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

func (r *registryClient) getSHAFromV2(registryURL, repositoryName, tag string, wg *sync.WaitGroup) (*domain.ManifestTag, error) {
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

	tagv2, err := r.getLastManifestForTagv2(registryURL, repositoryName, tag, wg)
	if err != nil {
		return nil, err
	}

	return &domain.ManifestTag{
		Tag:       tag,
		SHA:       tagv2.SHA,
		CreatedAt: lastDate,
	}, nil
}

func (r *registryClient) getLastManifestForTagv2(registryURL, repositoryName, tag string, wg *sync.WaitGroup) (*domain.ManifestTag, error) {
	defer wg.Done()

	c := client.New[any]()

	req, err := client.NewRequest(http.MethodGet, "https://"+registryURL+apiVersion+repositoryName+MANIFEST_ENDPOINT+tag, nil)
	if err != nil {
		return nil, err
	}

	r.AddHeader("Accept", headerV2)

	c.Transport = r.RoundTripper

	_, httpResponse, err := c.Do(req)
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

	shaDigest := httpResponse.Header.Get("Docker-Content-Digest")

	return &domain.ManifestTag{
		Tag: tag,
		SHA: shaDigest,
	}, nil
}

func getAuthenticationParams(realmHeader string) (realm string, service string) {
	realm = strings.Split(strings.Split(realmHeader, "realm=\"")[1], "\"")[0]
	service = strings.Split(strings.Split(realmHeader, "service=\"")[1], "\"")[0]
	return realm, service
}

func (r *registryClient) PullingImage(registryURL, repositoryName string) (domain.ManifestTag, error) {
	tags, err := r.listTags(registryURL, repositoryName)
	if err != nil {
		return domain.ManifestTag{}, err
	}

	results := make(chan *domain.ManifestTag, len(tags.Tags))

	var wg sync.WaitGroup
	for _, tag := range tags.Tags {
		wg.Add(1)

		go func(tag string) {
			manifest, err := r.getSHAFromV2(registryURL, repositoryName, tag, &wg)
			if err != nil {
				log.Fatal().Err(err).Msg("getting last manifest call failed")
				return
			}

			results <- manifest
		}(tag)
	}

	wg.Wait()

	var lastTagCreated domain.ManifestTag
	for manifest := range results {
		diff := manifest.CreatedAt.Sub(lastTagCreated.CreatedAt)

		if diff > 0 {
			lastTagCreated = *manifest
		}

		if len(results) == 0 {
			close(results)
		}
	}

	return lastTagCreated, nil
}
