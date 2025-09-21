output "ecr_repository_url" {
  value = aws_ecr_repository.repo.repository_url
}

output "alb_dns_name" {
  value = aws_lb.this.dns_name
}

output "service_name" {
  value = aws_ecs_service.this.name
}

