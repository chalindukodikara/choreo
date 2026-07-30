You can sign in to the AWS Management Console and upload, add, or edit a file to a repository directly from the AWS CodeCommit console. This is a quick way to make a change. However, if you want to work with multiple files, files across branches, and so on, consider setting up your local computer to work with repositories. The easiest way to set up CodeCommit is to configure HTTPS Git credentials for AWS CodeCommit. This HTTPS authentication method:

Uses a static user name and password.

Works with all operating systems supported by CodeCommit.

Is also compatible with integrated development environments (IDEs) and other development tools that support Git credentials.

You can use other methods if you do not want to or cannot use Git credentials for operational reasons. For example, if you access CodeCommit repositories using federated access, temporary credentials, or a web identity provider, you cannot use Git credentials. We recommend that you set up your local computer using the git-remote-codecommit command. Review these options carefully, to decide which alternative method works best for you.

Setting up using Git credentials

Setting up using other methods

Compatibility for CodeCommit, Git, and other components

For information about using CodeCommit and Amazon Virtual Private Cloud, see Using AWS CodeCommit with interface VPC endpoints .

View and manage your credentials

You can view and manage your CodeCommit credentials from the AWS console through My Security Credentials.

Note
This option is not available for users using federated access, temporary credentials, or a web identity provider.

Sign in to the AWS Management Console and open the IAM console at https://console.aws.amazon.com/iam/.

In the navigation bar on the upper right, choose your user name, and then choose Security Credentials.

Choose the AWS CodeCommit credentials tab.

Setting up using Git credentials

With HTTPS connections and Git credentials, you generate a static user name and password in IAM. You then use these credentials with Git and any third-party tool that supports Git user name and password authentication. This method is supported by most IDEs and development tools. It is the simplest and easiest connection method to use with CodeCommit.

For HTTPS users using Git credentials: Follow these instructions to set up connections between your local computer and CodeCommit repositories using Git credentials.

For connections from development tools: Follow these guidelines to set up connections between your IDE or other development tools and CodeCommit repositories using Git credentials. IDEs that support Git credentials include (but are not limited to) Visual Studio, Xcode, and IntelliJ.

Setting up using other methods

You can use the SSH protocol instead of HTTPS to connect to your CodeCommit repository. With SSH connections, you create public and private key files on your local machine that Git and CodeCommit use for SSH authentication. You associate the public key with your IAM user. You store the private key on your local machine. Because SSH requires manual creation and management of public and private key files, you might find Git credentials simpler and easier to use with CodeCommit.

Unlike Git credentials, SSH connection setup varies, depending on the operating system on your local computer.

For SSH users not using the AWS CLI: Follow these abbreviated instructions if you already have a public-private key pair and are familiar with SSH connections on your local computer.

For SSH connections on Linux, macOS, or Unix: Follow these instructions for a step-by-step walkthrough of creating a public-private key pair and setting up connections on Linux, macOS, or Unix operating systems.

For SSH connections on Windows: Follow these instructions for a step-by-step walkthrough of creating public-private key pair and setting up connections on Windows operating systems.

If you are connecting to CodeCommit and AWS using federated access, an identity provider, or temporary credentials, or if you do not want to configure IAM users or Git credentials for IAM users, you can set up connections to CodeCommit repositories in one of two ways:

Install and use git-remote-codecommit (recommended).

Install and use the credential helper included in the AWS CLI.

Both methods support accessing CodeCommit repositories without requiring an IAM user, which means that you can connect to repositories using federated access and temporary credentials. The git-remote-codecommit utility is the recommended approach. It extends Git and is compatible with a variety of Git versions and credential helpers. However, not all IDEs support the clone URL format used by git-remote-codecommit. You might have to manually clone repositories to your local computer before you can work with them in your IDE.

Follow the instructions in Setup Steps for HTTPS Connections to AWS CodeCommit Repositories with git-remote-codecommit to install and set up git-remote-codecommit on Windows, Linux, macOS, or Unix.

The credential helper included in the AWS CLI allows Git to use HTTPS and a cryptographically signed version of your IAM user credentials or Amazon EC2 instance role whenever Git needs to authenticate with AWS to interact with CodeCommit repositories. Some operating systems and Git versions have their own credential helpers, which conflict with the credential helper included in the AWS CLI. They can cause connectivity issues for CodeCommit.

For HTTPS connections on Linux, macOS, or Unix with the AWS CLI credential helper: Follow these instructions for a step-by-step walkthrough of installing and setting up the credential helper on Linux, macOS, or Unix systems.

For HTTPS connections on Windows with the AWS CLI credential helper: Follow these instructions for a step-by-step walkthrough of installing and setting up the credential helper on Windows systems.

If you are connecting to a CodeCommit repository that is hosted in another Amazon Web Services account, you can configure access and set up connections using roles, policies, and the credential helper included in the AWS CLI.

Configure cross-account access to an AWS CodeCommit repository using roles: Follow these instructions for a step-by-step walkthrough of configuring cross-account access in one Amazon Web Services account to users in an IAM group in another Amazon Web Services account.

Compatibility for CodeCommit, Git, and other components

When you work with CodeCommit, you use Git. You might use other programs, too. The following table provides the latest guidance for version compatibility. As a best practice, we recommend that you use the latest versions of Git and other software.

Version compatibility information for AWS CodeCommit
Component	Version
Git	CodeCommit supports Git versions 1.7.9 and later. Git version 2.28 supports configuring the branch name for initial commits. We recommend using a recent version of Git.
Curl	CodeCommit requires curl 7.33 and later. However, there is a known issue with HTTPS and curl update 7.41.0. For more information, see Troubleshooting.
Python (git-remote-codecommit only)	git-remote-codecommit requires version 3 and later.
Pip (git-remote-codecommit only)	git-remote-codecommit requires version 9.0.3 and later.
AWS CLI (git-remote-codecommit only)	We recommend a recent version of AWS CLI version 2 for all CodeCommit users. git-remote-codecommit requires AWS CLI version 2 to support AWS SSO and connections that require temporary credentials, such as federated users.

If you want to connect to CodeCommit using a root account, federated access, or temporary credentials, you should set up access using git-remote-codecommit. This utility provides a simple method for pushing and pulling code from CodeCommit repositories by extending Git. It is the recommended method for supporting connections made with federated access, identity providers, and temporary credentials. To assign permissions to a federated identity, you create a role and define permissions for the role. When a federated identity authenticates, the identity is associated with the role and is granted the permissions that are defined by the role. For information about roles for federation, see Create a role for a third-party identity provider (federation) in the IAM User Guide. If you use IAM Identity Center, you configure a permission set. To control what your identities can access after they authenticate, IAM Identity Center correlates the permission set to a role in IAM. For information about permissions sets, see Permission sets in the AWS IAM Identity Center User Guide.

You can also use git-remote-codecommit with an IAM user. Unlike other HTTPS connection methods, git-remote-codecommit does not require setting up Git credentials for the user.

Note
Some IDEs do not support the clone URL format used by git-remote-codecommit. You might have to manually clone repositories to your local computer before you can work with them in your preferred IDE. For more information, see Troubleshooting git-remote-codecommit and AWS CodeCommit.

These procedures are written with the assumption that you have an Amazon Web Services account, have created at least one repository in CodeCommit, and use an IAM user with a managed policy when connecting to CodeCommit repositories. For information about how to configure access for federated users and other rotating credential types, see Connecting to AWS CodeCommit repositories with rotating credentials.

Topics
Step 0: Install prerequisites for git-remote-codecommit

Step 1: Initial configuration for CodeCommit

Step 2: Install git-remote-codecommit

Step 3: Connect to the CodeCommit console and clone the repository

Next steps

Step 0: Install prerequisites for git-remote-codecommit

Before you can use git-remote-codecommit, you must install some prerequisites on your local computer. These include:

A supported version of Python. For more information about supported versions of Python, see git-remote-codecommit.

For more information about setting up and using git-remote-codecommit, see Setup steps for HTTPS connections to AWS CodeCommit with git-remote-codecommit.

Git

Note
When you install Python on Windows, make sure that you choose the option to add Python to the path.

git-remote-codecommit requires pip version 9.0.3 or later. To check your version of pip, open a terminal or command line and run the following command:


pip --version
You can run the following two commands to update your version of pip to the latest version:


curl -O https://bootstrap.pypa.io/get-pip.py
python3 get-pip.py --user
To work with files, commits, and other information in CodeCommit repositories, you must install Git on your local machine. CodeCommit supports Git versions 1.7.9 and later. Git version 2.28 supports configuring the branch name for initial commits. We recommend using a recent version of Git.

To install Git, we recommend websites such as Git Downloads.

Note
Git is an evolving, regularly updated platform. Occasionally, a feature change might affect the way it works with CodeCommit. If you encounter issues with a specific version of Git and CodeCommit, review the information in Troubleshooting.

Step 1: Initial configuration for CodeCommit

Follow these steps to create an IAM user, configure it with the appropriate policies, obtain an access key and secret key, and install and configure the AWS CLI.

To create and configure an IAM user for accessing CodeCommit
Create an Amazon Web Services account by going to http://aws.amazon.com and choosing Sign Up.

Create an IAM user, or use an existing one, in your Amazon Web Services account. Make sure you have an access key ID and a secret access key associated with that IAM user. For more information, see Creating an IAM User in Your Amazon Web Services account.

Note
CodeCommit requires AWS Key Management Service. If you are using an existing IAM user, make sure there are no policies attached to the user that expressly deny the AWS KMS actions required by CodeCommit. For more information, see AWS KMS and encryption.

Sign in to the AWS Management Console and open the IAM console at https://console.aws.amazon.com/iam/.

In the IAM console, in the navigation pane, choose Users, and then choose the IAM user you want to configure for CodeCommit access.

On the Permissions tab, choose Add Permissions.

In Grant permissions, choose Attach existing policies directly.

From the list of policies, select AWSCodeCommitPowerUser or another managed policy for CodeCommit access. For more information, see AWS managed policies for CodeCommit.

After you have selected the policy you want to attach, choose Next: Review to review the list of policies to attach to the IAM user. If the list is correct, choose Add permissions.

For more information about CodeCommit managed policies and sharing access to repositories with other groups and users, see Share a repository and Authentication and access control for AWS CodeCommit.

To install and configure the AWS CLI
On your local machine, download and install the AWS CLI. This is a prerequisite for interacting with CodeCommit from the command line. We recommend that you install AWS CLI version 2. It is the most recent major version of the AWS CLI and supports all of the latest features. It is the only version of the AWS CLI that supports using a root account, federated access, or temporary credentials with git-remote-codecommit.

For more information, see Getting Set Up with the AWS Command Line Interface.

Note
CodeCommit works only with AWS CLI versions 1.7.38 and later. As a best practice, install or upgrade the AWS CLI to the latest version available. To determine which version of the AWS CLI you have installed, run the aws --version command.

To upgrade an older version of the AWS CLI to the latest version, see Installing the AWS Command Line Interface.

Run this command to verify that the CodeCommit commands for the AWS CLI are installed.


aws codecommit help
This command returns a list of CodeCommit commands.

Configure the AWS CLI with a profile by using the configure command, as follows:.


aws configure
When prompted, specify the AWS access key and AWS secret access key of the IAM user to use with CodeCommit. Also, be sure to specify the AWS Region where the repository exists, such as us-east-2. When prompted for the default output format, specify json. For example, if you are configuring a profile for an IAM user:


AWS Access Key ID [None]: Type your IAM user AWS access key ID here, and then press Enter
AWS Secret Access Key [None]: Type your IAM user AWS secret access key here, and then press Enter
Default region name [None]: Type a supported region for CodeCommit here, and then press Enter
Default output format [None]: Type json here, and then press Enter
For more information about creating and configuring profiles to use with the AWS CLI, see the following:

Named Profiles

Using an IAM Role in the AWS CLI

Set command

Connecting to AWS CodeCommit repositories with rotating credentials

To connect to a repository or a resource in another AWS Region, you must reconfigure the AWS CLI with the default Region name. Supported default Region names for CodeCommit include:

us-east-2

us-east-1

eu-west-1

us-west-2

ap-northeast-1

ap-southeast-1

ap-southeast-2

ap-southeast-3

me-central-1

eu-central-1

ap-northeast-2

sa-east-1

us-west-1

eu-west-2

ap-south-1

ap-south-1

ca-central-1

us-gov-west-1

us-gov-east-1

eu-north-1

ap-east-1

me-south-1

cn-north-1

cn-northwest-1

eu-south-1

ap-northeast-3

af-south-1

il-central-1

For more information about CodeCommit and AWS Region, see Regions and Git connection endpoints. For more information about IAM, access keys, and secret keys, see How Do I Get Credentials? and Managing Access Keys for IAM Users. For more information about the AWS CLI and profiles, see Named Profiles.

Step 2: Install git-remote-codecommit

Follow these steps to install git-remote-codecommit.

To install git-remote-codecommit
At the terminal or command line, run the following command:


pip install git-remote-codecommit
Note
Depending on your operating system and configuration, you might need to run this command with elevated permissions, such as sudo, or use the --user parameter to install to a directory that doesn't require special privileges, such as your current user account. For example, on a computer running Linux, macOS, or Unix:


sudo pip install git-remote-codecommit
On a computer running Windows:


pip install --user git-remote-codecommit
Monitor the installation process until you see a success message.

Step 3: Connect to the CodeCommit console and clone the repository

If an administrator has already sent you the clone URL to use with git-remote-codecommit for the CodeCommit repository, you can skip connecting to the console and clone the repository directly.

To connect to a CodeCommit repository
Open the CodeCommit console at https://console.aws.amazon.com/codesuite/codecommit/home.

In the region selector, choose the AWS Region where the repository was created. Repositories are specific to an AWS Region. For more information, see Regions and Git connection endpoints.

Find the repository you want to connect to from the list and choose it. Choose Clone URL, and then choose the protocol you want to use when cloning or connecting to the repository. This copies the clone URL.

Copy the HTTPS URL if you are using either Git credentials with your IAM user or the credential helper included with the AWS CLI.

Copy the HTTPS (GRC) URL if you are using the git-remote-codecommit command on your local computer.

Copy the SSH URL if you are using an SSH public/private key pair with your IAM user.

Note
If you see a Welcome page instead of a list of repositories, there are no repositories associated with your AWS account in the AWS Region where you are signed in. To create a repository, see Create an AWS CodeCommit repository or follow the steps in the Getting started with Git and CodeCommit tutorial.

At the terminal or command prompt, clone the repository with the git clone command. Use the HTTPS git-remote-codecommit URL you copied and the name of the AWS CLI profile, if you created a named profile. If you do not specify a profile, the command assumes the default profile. The local repo is created in a subdirectory of the directory where you run the command. For example, to clone a repository named MyDemoRepo to a local repo named my-demo-repo:


git clone codecommit://MyDemoRepo my-demo-repo
To clone the same repository using a profile named CodeCommitProfile:


git clone codecommit://CodeCommitProfile@MyDemoRepo my-demo-repo
To clone a repository in a different AWS Region than the one configured in your profile, include the AWS Region name. For example:


git clone codecommit::ap-northeast-1://MyDemoRepo my-demo-repo
Next steps

You have completed the prerequisites. Follow the steps in Getting started with CodeCommit to start using CodeCommit.

To learn how to create and push your first commit, see Create a commit in AWS CodeCommit. If you're new to Git, you might also want to review the information in Where can I learn more about Git? and Getting started with Git and AWS CodeCommit.

Configure cross-account access to an AWS CodeCommit repository using roles
 PDF
 RSS
Focus mode
You can configure access to CodeCommit repositories for IAM users and groups in another AWS account. This is often referred to as cross-account access. This section provides examples and step-by-step instructions for configuring cross-account access for a repository named MySharedDemoRepo in the US East (Ohio) Region in an AWS account (referred to as AccountA) to IAM users who belong to an IAM group named DevelopersWithCrossAccountRepositoryAccess in another AWS account (referred to as AccountB).

This section is divided into three parts:

Actions for the Administrator in AccountA.

Actions for the Administrator in AccountB.

Actions for the repository user in AccountB.

To configure cross-account access:

The administrator in AccountA signs in as an IAM user with the permissions required to create and manage repositories in CodeCommit and create roles in IAM. If you are using managed policies, apply IAMFullAccess and AWSCodeCommitFullAccess to this IAM user.

The example account ID for AccountA is 111122223333.

The administrator in AccountB signs in as an IAM user with the permissions required to create and manage IAM users and groups, and to configure policies for users and groups. If you are using managed policies, apply IAMFullAccess to this IAM user.

The example account ID for AccountB is 888888888888.

The repository user in AccountB, to emulate the activities of a developer, signs in as an IAM user who is a member of the IAM group created to allow access to the CodeCommit repository in AccountA. This account must be configured with:

AWS Management Console access.

An access key and secret key to use when connecting to AWS resources and the ARN of the role to assume when accessing repositories in AccountA.

The git-remote-codecommit utility on the local computer where the repository is cloned. This utility requires Python and its installer, pip. You can download the utility from git-remote-codecommit on the Python Package Index website.

For more information, see Setup steps for HTTPS connections to AWS CodeCommit with git-remote-codecommit and IAM users.

Topics
Cross-account repository access: Actions for the administrator in AccountA

Cross-account repository access: Actions for the administrator in AccountB

Cross-account repository access: Actions for the repository user in AccountB

Cross-account repository access: Actions for the administrator in AccountA
 PDF
 RSS
Focus mode
To allow users or groups in AccountB to access a repository in AccountA, an AccountA administrator must:

Create a policy in AccountA that grants access to the repository.

Create a role in AccountA that can be assumed by IAM users and groups in AccountB.

Attach the policy to the role.

The following sections provide steps and examples.

Topics
Step 1: Create a policy for repository access in AccountA

Step 2: Create a role for repository access in AccountA

Step 1: Create a policy for repository access in AccountA

You can create a policy in AccountA that grants access to the repository in AccountA to users in AccountB. Depending on the level of access you want to allow, do one of the following:

Configure the policy to allow AccountB users access to a specific repository, but do not allow them to view a list of all repositories in AccountA.

Configure additional access to allow AccountB users to choose the repository from a list of all repositories in AccountA.

To create a policy for repository access
Sign in to the AWS Management Console as an IAM user with permissions to create policies in AccountA.

Open the IAM console at https://console.aws.amazon.com/iam/.

In the navigation pane, choose Policies.

Choose Create policy.

Choose the JSON tab, and paste the following JSON policy document into the JSON text box. Replace us-east-2 with the AWS Region for the repository, 111122223333 with the account ID for AccountA, and MySharedDemoRepo with the name for your CodeCommit repository in AccountA:


JSON

{
"Version":"2012-10-17",
"Statement": [
    {
        "Effect": "Allow",
        "Action": [
            "codecommit:BatchGet*",
            "codecommit:Create*",
            "codecommit:DeleteBranch",
            "codecommit:Get*",
            "codecommit:List*",
            "codecommit:Describe*",
            "codecommit:Put*",
            "codecommit:Post*",
            "codecommit:Merge*",
            "codecommit:Test*",
            "codecommit:Update*",
            "codecommit:GitPull",
            "codecommit:GitPush"
        ],
        "Resource": [
            "arn:aws:codecommit:us-east-2:111122223333:MySharedDemoRepo"
        ]
    }
]
}


Provide feedback
If you want users who assume this role to be able to view a list of repositories on the CodeCommit console home page, add an additional statement to the policy, as follows:


JSON

{
    "Version":"2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "codecommit:BatchGet*",
                "codecommit:Create*",
                "codecommit:DeleteBranch",
                "codecommit:Get*",
                "codecommit:List*",
                "codecommit:Describe*",
                "codecommit:Put*",
                "codecommit:Post*",
                "codecommit:Merge*",
                "codecommit:Test*",
                "codecommit:Update*",
                "codecommit:GitPull",
                "codecommit:GitPush"
            ],
            "Resource": [
                "arn:aws:codecommit:us-east-2:111122223333:MySharedDemoRepo"
            ]
        },
        {
            "Effect": "Allow",
            "Action": "codecommit:ListRepositories",
            "Resource": "*"
        }
    ]
}


Provide feedback
This access makes it easier for users who assume this role with this policy to find the repository to which they have access. They can choose the name of the repository from the list and be directed to the home page of the shared repository (Code). Users cannot access any of the other repositories they see in the list, but they can view the repositories in AccountA on the Dashboard page.

If you do not want to allow users who assume the role to be able to view a list of all repositories in AccountA, use the first policy example, but make sure that you send those users a direct link to the home page of the shared repository in the CodeCommit console.

Choose Review policy. The policy validator reports syntax errors (for example, if you forget to replace the example Amazon Web Services account ID and repository name with your Amazon Web Services account ID and repository name).

On the Review policy page, enter a name for the policy (for example, CrossAccountAccessForMySharedDemoRepo). You can also provide an optional description for this policy. Choose Create policy.

Step 2: Create a role for repository access in AccountA

After you have configured a policy, create a role that IAM users and groups in AccountB can assume, and attach the policy to that role.

To create a role for repository access
In the IAM console, choose Roles.

Choose Create role.

Choose Another Amazon Web Services account.

In Account ID, enter the Amazon Web Services account ID for AccountB (for example, 888888888888). Choose Next: Permissions.

In Attach permissions policies, select the policy you created in the previous procedure (CrossAccountAccessForMySharedDemoRepo). Choose Next: Review.

In Role name, enter a name for the role (for example, MyCrossAccountRepositoryContributorRole). You can also enter an optional description to help others understand the purpose of the role.

Choose Create role.

Open the role you just created, and copy the role ARN (for example, arn:aws:iam::111122223333:role/MyCrossAccountRepositoryContributorRole). You need to provide this ARN to the AccountB administrator.
