Role's trust relationship
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": [
                    "arn:aws:iam::356064724012:root",
                    "arn:aws:iam::356064724012:user/chalindu"
                ]
            },
            "Action": "sts:AssumeRole",
            "Condition": {}
        }
    ]
}

Added this inline permission to user
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

This access credentials belong to this chalindu user :samples/try-out-2/2-codecommit/secret.yaml.
