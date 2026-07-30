### Support AWS Codecommit through the git-remote-codecommit

Setup steps for HTTPS connections to AWS CodeCommit with git-remote-codecommit
 PDF
 RSS
Focus mode
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

1. Update "samples/getting-started/workflow-templates/checkout-source.yaml" support this.
2. Update samples/getting-started/ci-workflows/* to support this since this might require adding parameters to get from user.
3. Add a guide to create a component with this.