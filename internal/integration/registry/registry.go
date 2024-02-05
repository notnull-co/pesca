package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	apiVersion         = "v2"
	LIST_TAGS_ENDPOINT = "/tags/list"
	MANIFEST_ENDPOINT  = "/manifests"
)

type registryClient struct {
	httpClient *http.Client
}

type RegistryClient interface {
	ListTags(registryURL, repositoryName, orderType string) ([]string, error)
	GetManifest(registryURL, repositoryName, tag string) (string, error)
}

func New() RegistryClient {
	return &registryClient{
		httpClient: http.DefaultClient,
	}
}

func (r *registryClient) ListTags(registryURL, repositoryName, orderType string) ([]string, error) {
	r.httpClient.Get(fmt.Sprintf("%s/%s/%s%s", registryURL, apiVersion, repositoryName, LIST_TAGS_ENDPOINT))
	return nil, nil
}

func (r *registryClient) GetManifest(registryURL, repositoryName, tag string) (string, error) {
	r.httpClient.Get(fmt.Sprintf("%s/%s/%s/%s/%s", registryURL, apiVersion, repositoryName, MANIFEST_ENDPOINT, tag))
	return "", nil
}

func (r *registryClient) sendRequest(registryUrl, repositoryName, endpoint, token string) (*http.Response, error) {

	client := http.Client{}
	req, err := http.NewRequest("GET", registryUrl+"/"+apiVersion+"/"+repositoryName+endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header = http.Header{
		"Accept":        {"application/vnd.docker.distribution.manifest.v1+prettyjws"},
		"Authorization": {"Bearer " + token},
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *registryClient) getToken(registryUrl, repositoryName string) (string, error) {
	realm, service, err := r.getAuthenticationParams(registryUrl, repositoryName)
	if err != nil {
		return "", err
	}

	tokenResponse, err := r.httpClient.Get(realm + "?service=" + service + "&scope=repository:" + repositoryName + ":pull")
	if err != nil {
		return "", err
	}

	data := make(map[string]interface{})
	err = json.NewDecoder(tokenResponse.Body).Decode(&data)
	if err != nil {
		return "", err
	}
	if token, ok := data["token"].(string); ok {
		return token, nil
	} else {
		return "", err
	}
}

func (r *registryClient) getAuthenticationParams(registryUrl string, repositoryName string) (realm string, service string, err error) {

	response, err := r.httpClient.Head(registryUrl + "/" + apiVersion + "/")
	if err != nil {
		return "", "", err
	}

	realmHeader := response.Header.Get("WWW-Authenticate")
	realm = strings.Split(strings.Split(realmHeader, "realm=\"")[1], "\"")[0]
	service = strings.Split(strings.Split(realmHeader, "service=\"")[1], "\"")[0]
	return realm, service, nil
}
