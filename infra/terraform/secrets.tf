resource "aws_secretsmanager_secret" "db_password" {
  name                    = "${local.name}/db-password"
  recovery_window_in_days = var.environment == "production" ? 30 : 0
  tags                    = local.tags
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id     = aws_secretsmanager_secret.db_password.id
  secret_string = random_password.db_password.result
}

resource "aws_secretsmanager_secret" "jwt_secret" {
  name                    = "${local.name}/jwt-secret"
  recovery_window_in_days = var.environment == "production" ? 30 : 0
  tags                    = local.tags
}

resource "aws_secretsmanager_secret_version" "jwt_secret" {
  secret_id     = aws_secretsmanager_secret.jwt_secret.id
  secret_string = random_password.jwt_secret.result
}

resource "random_password" "jwt_secret" {
  length  = 64
  special = false
}

resource "aws_secretsmanager_secret" "api_keys" {
  name                    = "${local.name}/api-keys"
  recovery_window_in_days = var.environment == "production" ? 30 : 0
  tags                    = local.tags
}

# Note: Populate api-keys secret manually after creation:
# aws secretsmanager put-secret-value --secret-id cryptofunk-dev/api-keys \
#   --secret-string '{"ANTHROPIC_API_KEY":"...","OPENAI_API_KEY":"...","BINANCE_API_KEY":"...","BINANCE_API_SECRET":"..."}'
