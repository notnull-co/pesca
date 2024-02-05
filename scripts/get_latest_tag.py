import requests
import json
from datetime import datetime

REGISTRY_URL = "https://gcr.io"
REPOSITORY_NAME = "kubeflow-images-public/admission-webhook"

# REGISTRY_URL = "https://registry.hub.docker.com"
# REPOSITORY_NAME = "murillovaz/minecraft-server"

def get_time(time_str):
    try:
        date_parts = time_str.split('.')
        date_part = date_parts[0]
        return datetime.strptime(date_part, "%Y-%m-%dT%H:%M:%S")
    except:
        return None

def get_token(registry_url, repo_name):
    response = requests.head(f"{registry_url}/v2/")
    realm_header = response.headers.get("WWW-Authenticate", "")

    realm = realm_header.split('realm="')[1].split('"')[0]
    service = realm_header.split('service="')[1].split('"')[0]
    token_response = requests.get(f"{realm}?service={service}&scope=repository:{repo_name}:pull")
    return token_response.json()['token']

def get_repo_tags(registry_url, repo_name, token_header):
    repo_tags = requests.get(f"{registry_url}/v2/{repo_name}/tags/list", headers=token_header)
    return repo_tags.json()

def get_tag_manifest(registry_url, repo_name, tag, token_header):
    manifest_response = requests.get(f"{registry_url}/v2/{repo_name}/manifests/{tag}", headers=token_header)
    return manifest_response.json()

def main():
    token = get_token(REGISTRY_URL, REPOSITORY_NAME)
    headers = {"Authorization": f"Bearer {token}"}
    # this ensures that the schema v1 is the one to be used
    headers['Accept'] = "application/vnd.docker.distribution.manifest.v1+prettyjws"

    tags = get_repo_tags(REGISTRY_URL, REPOSITORY_NAME, headers)

    for tag in tags['tags']:
        manifest_response_json = get_tag_manifest(REGISTRY_URL, REPOSITORY_NAME, tag, headers)

        last_update = None    
        for history in manifest_response_json['history']:
            created = get_time(json.loads(history['v1Compatibility'])['created'])
            
            if last_update is None or (created is not None and created > last_update):
                last_update = created
                
        print(f"Tag: {tag} - Last Updated: {last_update}")

main()