variable "db_name" {
  description = "RDS database name"
  type        = string
}

variable "db_username" {
  description = "RDS database username"
  type        = string
}

variable "db_password" {
  description = "RDS database password"
  type        = string
  sensitive   = true
}

variable "jwt_secret" {
  description = "JWT signing secret"
  type        = string
  sensitive   = true
}

variable "alerts_email" {
  description = "Email that receives alerts"
  type        = string
}

variable "ca_cert" {
  description = "CA Certificate"
  type        = string
  sensitive   = true
}

variable "server_cert" {
  description = "Server certificate"
  type        = string
  sensitive   = true
}

variable "server_key" {
  description = "Server key"
  type        = string
  sensitive   = true
}
