Currently, workflowrun controller read the argo workflow and create the workload CR if it has the generate-workload-cr step and workload-cr output. This is a special case to simplify the workload creation process.

We need to avoid that. I am trying to modify the clusterworkflowtemplate to call the api server to create the workload instead of generating the workload CR and let the controller read it and create the workload. So we can remove that special case logic in the controller and also remove the condition related to that.

We will support this considering thunder as the IDP.

1. Add a new app for workflows in thunder initialization
- install/k3d/common/values-thunder.yaml

2. Update the clusterworkflowtemplate to call the thunder oauth api to get the token
- samples/getting-started/workflow-templates/publish-image-k3d.yaml
- samples/getting-started/workflow-templates/publish-image.yaml

2.1 Find the call api in k3d case "http://thunder.openchoreo.localhost:8080/oauth2/token".
This url is specific to the localhost environment, for k3d we can access using something like "host.k3d.internal:10082".
You have access to the cluster, check using kubectl to find the url.

2.2 call the oauth api to get the token using client id and password, hardcode those 2 and add a comment to mount the passowrd as a secret
and add a comment to update the oauth server url when you move to another identity provider or when the url changes.

3. Call the api server with the created workload CR using the occ to create the workload in the control plane.

3.1 Find the openchoreo api server url, it should be something like "host.k3d.internal:10080" for k3d, check using kubectl since you have access to the cluster.

3.2 Call the api server to create the workload, using curl
- openapi/openchoreo-api.yaml
- /api/v1/namespaces/{namespaceName}/workloads:

kubectl run curl-check2 --image=curlimages/curl:latest --restart=Never --command -- sh -c "echo '--- API well-known ---'; curl -s -H 'Host: api.openchoreo.localhost' http://host.k3d.internal:8080/.well-known/oauth-protected-resource 2>&1; echo; echo '--- API namespaces ---'; curl -s -o /dev/null -w 'HTTP %{http_code}' -H 'Host: api.openchoreo.localhost'
http://host.k3d.internal:8080/api/v1/namespaces 2>&1; echo; echo '--- Thunder token ---'; curl -s -X POST -H 'Host: thunder.openchoreo.localhost' -H 'Content-Type: application/x-www-form-urlencoded' -d 'grant_type=client_credentials&client_id=openchoreo-system-app&client_secret=openchoreo-system-app-secret' http://host.k3d.internal:8080/oauth2/token 2>&1; echo; sleep 3" 2>/dev/null; sleep 8; kubectl logs
curl-check2 2>/dev/null; kubectl delete pod curl-check2 --force 2>/dev/null
