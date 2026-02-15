# CryptoFunk Terraform Infrastructure

AWS EKS infrastructure for the CryptoFunk trading platform.

## Prerequisites

- [Terraform](https://www.terraform.io/downloads) >= 1.7
- [AWS CLI](https://aws.amazon.com/cli/) configured with appropriate credentials
- S3 bucket + DynamoDB table for state backend (see below)

## State Backend Setup

Create the S3 bucket and DynamoDB table for Terraform state locking:

```bash
# Create S3 bucket
aws s3api create-bucket --bucket cryptofunk-terraform-state --region us-east-1

# Enable versioning
aws s3api put-bucket-versioning --bucket cryptofunk-terraform-state \
  --versioning-configuration Status=Enabled

# Create DynamoDB table for locking
aws dynamodb create-table \
  --table-name cryptofunk-terraform-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```

Then uncomment the backend block in `main.tf`.

## Usage

```bash
# Copy and edit variables
cp terraform.tfvars.example terraform.tfvars

# Initialize
terraform init

# Preview changes
terraform plan

# Apply
terraform apply

# Configure kubectl
$(terraform output -raw configure_kubectl)
```

## Resources Created

| Resource | Description |
|----------|-------------|
| VPC | 3 AZs, public/private subnets, NAT gateway |
| EKS | Kubernetes 1.31 cluster with managed node groups |
| ECR | Container registries for all services |
| RDS | PostgreSQL 17 (TimescaleDB compatible) |
| ElastiCache | Redis 7 cluster |
| Secrets Manager | DB password, JWT secret, API keys |
| IAM | IRSA roles for pod-level AWS access |

## Environments

- **dev**: Single NAT gateway, smaller instances, no multi-AZ RDS
- **production**: Multi-AZ NAT + RDS, larger instances, backups enabled
