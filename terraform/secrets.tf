resource "aws_secretsmanager_secret" "db_password" {
  name = "realworld/db-password"

  tags = { Name = "realworld-db-password" }
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id     = aws_secretsmanager_secret.db_password.id
  secret_string = var.db_password
}

resource "aws_secretsmanager_secret" "jwt_secret" {
  name = "realworld/jwt-secret"

  tags = { Name = "realworld-jwt-secret" }
}

resource "aws_secretsmanager_secret_version" "jwt_secret" {
  secret_id     = aws_secretsmanager_secret.jwt_secret.id
  secret_string = var.jwt_secret
}

resource "aws_secretsmanager_secret" "ca_cert" {
  name = "realworld/ca-cert"

  tags = { Name = "realworld-ca-cert" }
}

resource "aws_secretsmanager_secret_version" "ca_cert" {
  secret_id     = aws_secretsmanager_secret.ca_cert.id
  secret_string = var.ca_cert
}

resource "aws_secretsmanager_secret" "server_cert" {
  name = "realworld/server-cert"

  tags = { Name = "realworld-server-cert" }
}

resource "aws_secretsmanager_secret_version" "server_cert" {
  secret_id     = aws_secretsmanager_secret.server_cert.id
  secret_string = var.server_cert
}

resource "aws_secretsmanager_secret" "server_key" {
  name = "realworld/server-key"

  tags = { Name = "realworld-server-key" }
}

resource "aws_secretsmanager_secret_version" "server_key" {
  secret_id     = aws_secretsmanager_secret.server_key.id
  secret_string = var.server_key
}
