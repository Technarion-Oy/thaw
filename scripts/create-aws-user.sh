AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

aws iam create-user --user-name github-actions-user-for-release-notes

aws iam create-policy --policy-name s3-access-for-github-user-release --description "This policy is used for Github actions to push static files into s3." --policy-document file://scripts/aws-s3-policy-doc.json

aws iam attach-user-policy --user-name github-actions-user-for-release-notes --policy-arn arn:aws:iam::$AWS_ACCOUNT_ID:policy/s3-access-for-github-user

aws iam create-access-key --user-name github-actions-user-for-release-notes