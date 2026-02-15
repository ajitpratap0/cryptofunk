resource "aws_elasticache_subnet_group" "main" {
  name       = local.name
  subnet_ids = module.vpc.private_subnets
  tags       = local.tags
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = local.name
  description          = "CryptoFunk Redis - ${var.environment}"

  engine         = "redis"
  engine_version = "7.1"
  node_type      = local.redis_node_type
  port           = 6379

  num_cache_clusters = var.environment == "production" ? 2 : 1

  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [aws_security_group.redis.id]

  at_rest_encryption_enabled = true
  transit_encryption_enabled = false # Set true if clients support TLS

  automatic_failover_enabled = var.environment == "production"

  snapshot_retention_limit = var.environment == "production" ? 3 : 0
  snapshot_window          = "04:00-05:00"
  maintenance_window       = "mon:05:00-mon:06:00"

  tags = local.tags
}
