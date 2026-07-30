Currently, there is no functionality to create a git secret through the OpenChoreo api server.

We need to add an API to get secret information such as secretName, pat. (later extend to ssh key)

api-path: api.HandleFunc("CREATE "+v1+"/namespace/{namespaceName}/git-secrets", h.CreateGitSecret)
body: token
internal/openchoreo-api/handlers/handlers.go

It should retrieve the buildplane CR in the namespace. Create a client.
Through the cluster gateway client, it should create a k8s secret with basic-auth type in the build plane "openchoreo-system" namespace. If namespace "openchoreo-system" doesnt exist, create it.
samples/try-out/git-secret-creation/secret.yaml

Then, create a push secret to push to the key-vault in the same "openchoreo-system". Build plane has which key-vault it is using to push. 
samples/try-out/git-secret-creation/push-secret.yaml

Then, create a secretReference in the {namespaceName} namespace.
samples/try-out/git-secret-creation/secret-reference.yaml
