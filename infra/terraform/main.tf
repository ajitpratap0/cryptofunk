# CryptoFunk Infrastructure - AWS EKS
#
# State backend configuration (uncomment after creating S3 bucket + DynamoDB table):
# terraform {
#   backend "s3" {
#     bucket         = "cryptofunk-terraform-state"
#     key            = "infra/terraform.tfstate"
#     region         = "us-east-1"
#     dynamodb_table = "cryptofunk-terraform-locks"
#     encrypt        = true
#   }
# }

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "cryptofunk"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

provider "kubernetes" {
  host                   = module.eks.cluster_endpoint
  cluster_ca_certificate = base64decode(module.eks.cluster_certificate_authority_data)

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "aws"
    args        = ["eks", "get-token", "--cluster-name", module.eks.cluster_name]
  }
}

data "aws_availability_zones" "available" {
  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}

locals {
  name   = "cryptofunk-${var.environment}"
  azs    = slice(data.aws_availability_zones.available.names, 0, 3)

  tags = {
    Project     = "cryptofunk"
    Environment = var.environment
  }
}
