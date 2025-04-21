
# YAFAI Skills 🚀 - Tools and integrations for YAFAI.

## Yafai Skills is the Tools Framework for YAFAI Agents

### Description

YAFAI skills is the tools framework for extending capabilities to YAFAI agents.

### Features

- REST API based
- Auth supported through API token, can piggy back on exisiting RBAC.

### Installation

```bash
brew tap yafai-hub/yafai
brew install yafai-skill

```

### Parameters to Skill Engine

Run the binary with the following parameters to start Skill Engin

```
//run the skill engine with below params

yafai-skill -m [manifest file path] -k [api key for the service]
yafai-skill -h //for help on parameters.

```

### Skill Manifest File

Skill manifest file is the center point for defining the skill server, below is the v0.0.1 version of the manifest file

```yaml
name: ServiceName
description: YAFAI skills for ServiceName
actions:
  ACtion1:
    desc: Action Description
    base_url: Service URL with place holders Eg. service.com/{url_path}/{query_param}
    method: POST
    params:
      - {name: test-param1, type: string, in: path, desc: "Param1.", required: true}
      - {name: test-param2, type: string, in: query, desc: "Param2.", required: true}
      - {name: test-param3, type: string, in: body, desc: "Param3", required: true}
    response_template: #golang text templates for preparing response
      success: "Completed action with {{.response}}'"
      failure: "Failed to complete action : {{.Error}}"

```

### Pre build Manifests Coming Soon!!

### License

Apache 2.0 License - see LICENSE.md for details

###