In workflow run controller, we create the service account called workflow-sa

And we expect user to provide the service account and we don't need to get it from the user.

In the admission/mutating webhook, we can extract the run template and add/replace the service account field with the same name.


