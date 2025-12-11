# CryptoFunk Vault Configuration
# This is the configuration for development mode
# For production, use a proper backend like Consul or PostgreSQL

ui = true
disable_mlock = true

storage "file" {
  path = "/vault/data"
}

listener "tcp" {
  address     = "0.0.0.0:8200"
  tls_disable = 1
}

api_addr = "http://0.0.0.0:8200"
