# Build a Component from AWS CodeCommit (git-remote-codecommit + IAM Role)

How to wire an OpenChoreo Component to an AWS CodeCommit repository using the
[`git-remote-codecommit`](https://github.com/aws/git-remote-codecommit) helper
with **IAM role assumption**. Instead of granting CodeCommit permissions
directly to an IAM user, this approach stores minimal base credentials (with
only `sts:AssumeRole` permission) and delegates CodeCommit access to an IAM
role. This follows the principle of least privilege and is the recommended
pattern for production setups.

## How it works

`git-remote-codecommit` is a Python helper that registers the `codecommit://`
URL scheme with Git. When Git is asked to fetch from `codecommit://MyRepo`,
the helper signs each HTTPS request with the caller's AWS credentials using
SigV4.

OpenChoreo's `checkout-source` workflow template detects the `codecommit://`
scheme on the repository URL, installs `git-remote-codecommit` on demand, and
materialises `~/.aws/credentials` from a Kubernetes secret that the build
workflow projects into the checkout container.

When `aws-role-arn` is present in the secret, checkout-source creates a named
AWS profile (`codecommit-role`) that assumes the given IAM role using the base
credentials. The repository URL is automatically rewritten to use this profile
(e.g. `codecommit://codecommit-role@MyRepo`), so boto3 handles the
`AssumeRole` call transparently.

```
SecretReference (control plane)
  └─ ExternalSecret (workflow plane, rendered by ci-workflow)
       └─ Kubernetes Secret with keys:
            aws-access-key-id / aws-secret-access-key (base creds, sts:AssumeRole only)
            aws-role-arn (IAM role with CodeCommit permissions)
            aws-region (optional)
              └─ mounted into checkout-source container at /etc/secrets/git-secret
                   └─ ~/.aws/credentials  ← base credentials ([default] profile)
                   └─ ~/.aws/config       ← [profile codecommit-role] with role_arn
                        └─ git clone codecommit://codecommit-role@MyRepo
                             └─ boto3 calls sts:AssumeRole, then signs CodeCommit requests
```

## Prerequisites

1. **AWS account** with at least one CodeCommit repository (e.g. `MyDemoRepo`
   in `us-east-1`).
2. **IAM role** (e.g. `arn:aws:iam::356064724012:role/Developer`) with
   `AWSCodeCommitPowerUser` (or a narrower equivalent) attached.
3. **IAM user** with a policy that allows `sts:AssumeRole` on the role above.
   For example, attach an inline policy like:
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": "sts:AssumeRole",
         "Resource": "arn:aws:iam::356064724012:role/Developer"
       }
     ]
   }
   ```
   Generate a long-lived access key pair for this user.
4. **OpenChoreo single-cluster k3d setup** as per
   <https://openchoreo.dev/docs/getting-started/try-it-out/on-k3d-locally/>,
   with the default ClusterSecretStore wired to your KV store (e.g. OpenBao).

## Step 1 — Store the AWS credentials and role ARN in your KV store

The `secret-reference.yaml` in this directory expects four KV entries:
`codecommit-access-key`, `codecommit-secret-key`, `codecommit-region`, and
`codecommit-role-arn`. Seed them with the values from your IAM user and role:

```bash
kubectl exec -n openbao openbao-0 -- sh -c '
  export BAO_ADDR=http://127.0.0.1:8200 BAO_TOKEN=root
  bao kv put secret/codecommit-access-key value="AKIA..."
  bao kv put secret/codecommit-secret-key value="..."
  bao kv put secret/codecommit-region     value="us-east-1"
  bao kv put secret/codecommit-role-arn   value="arn:aws:iam::356064724012:role/Developer"
'
```

> If you are using temporary / federated credentials, also seed
> `codecommit-session-token` and add an entry to the SecretReference
> mapping it to the secret key `aws-session-token`.

## Step 2 — Apply the SecretReference

```bash
kubectl apply -f samples/try-out-2/2-codecommit/secret-reference.yaml
```

This declares which KV entries to project into the build-time secret and which
secret keys (`aws-access-key-id`, `aws-secret-access-key`, `aws-region`) to
expose them under. The checkout script in
`samples/getting-started/workflow-templates/checkout-source.yaml` reads
exactly those keys.

## Step 3 — Apply the Component and trigger a build

```bash
kubectl apply -f samples/try-out-2/2-codecommit/component.yaml
```

The bundled WorkflowRun kicks off a build immediately. The component points at
`codecommit::us-east-1://MyDemoRepo` and at the SecretReference from step 2 —
edit both to match your repo and credentials.

Watch the build:

```bash
kubectl logs -n default-build -l workflows.argoproj.io/workflow=codecommit-greeter-build-01 \
  -c main -f --max-log-requests 10
```

You should see in the `checkout-source` step:

```
>> Repository: codecommit::us-east-1://MyDemoRepo
>> Detected AWS CodeCommit URL (git-remote-codecommit format)
>> Installing git-remote-codecommit
>> Authentication: AWS access key (git-remote-codecommit)
>> Using IAM role assumption: arn:aws:iam::356064724012:role/Developer
>> Rewrote URL to use profile: codecommit::us-east-1://codecommit-role@MyDemoRepo
>> Checked out branch: main (commit: <sha>)
```

## Supported URL formats

| URL                                          | Behaviour                                    |
|---|---|
| `codecommit://MyRepo`                        | Uses `region` from `~/.aws/config`.          |
| `codecommit::us-east-1://MyRepo`             | Explicit region (wins over config).          |
| `codecommit://MyProfile@MyRepo`              | Uses the named profile from `~/.aws/credentials`. The default `[default]` profile is what this sample produces. |

## Secret key reference

The checkout script looks for the following keys inside the mounted git secret:

| Secret key             | Required | Purpose                                             |
|---|---|---|
| `aws-access-key-id`    | Yes      | `[default] aws_access_key_id` in `~/.aws/credentials` |
| `aws-secret-access-key`| Yes      | `[default] aws_secret_access_key`                   |
| `aws-role-arn`         | No       | IAM role ARN to assume (creates `[profile codecommit-role]` with `role_arn` and `source_profile = default`; URL is rewritten to `codecommit://codecommit-role@Repo`) |
| `aws-session-token`    | No       | `[default] aws_session_token` (for STS / federated) |
| `aws-region`           | No       | `[default] region` in `~/.aws/config` (defaults to `us-east-1`) |

## Troubleshooting

- **`fatal: Unable to find remote helper for 'codecommit'`** — the install of
  `git-remote-codecommit` failed. Check the `apk add python3 py3-pip` and
  `pip3 install` lines in the checkout-source logs.
- **`botocore.exceptions.NoCredentialsError`** — the secret was projected
  without `aws-access-key-id` / `aws-secret-access-key`. Re-check the
  SecretReference `data[].secretKey` names.
- **`AccessDeniedException` on `GitPull`** — the assumed IAM role lacks
  CodeCommit permissions; attach `AWSCodeCommitPowerUser` (or equivalent) to
  the role, not the base IAM user.
- **`AccessDenied` on `sts:AssumeRole`** — the base IAM user lacks permission
  to assume the role, or the role's trust policy does not allow the user.
  Verify the role's trust policy includes the user's account/ARN, and that
  the user has an `sts:AssumeRole` policy for the role ARN.
- **`UnrecognizedClientException`** — wrong region. Either fix the URL
  (`codecommit::<region>://...`) or set `aws-region` in the secret.
