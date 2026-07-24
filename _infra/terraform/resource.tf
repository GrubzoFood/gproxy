#############################################
# Terraform & Provider
#############################################

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  backend "s3" {
    bucket       = "gproxy-tfstate"
    key          = "gproxy/terraform.tfstate"
    region       = "ap-south-2"
    encrypt      = true
    use_lockfile = true
  }
}

provider "aws" {
  region = "ap-south-2"
}

#############################################
# Use Default VPC
#############################################

data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

#############################################
# Latest Ubuntu 24.04 LTS AMI
#############################################

data "aws_ami" "ubuntu" {
  most_recent = true

  owners = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }
}

#############################################
# Security Group
#############################################

resource "aws_security_group" "gproxy" {
  name        = "gproxy"
  description = "Security Group for gproxy"
  vpc_id      = data.aws_vpc.default.id

  # HTTP
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HTTPS
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # No SSH required because deployments happen using AWS SSM

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

#############################################
# IAM Role for EC2
#############################################

resource "aws_iam_role" "gproxy" {
  name = "gproxy-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"

    Statement = [
      {
        Effect = "Allow"

        Principal = {
          Service = "ec2.amazonaws.com"
        }

        Action = "sts:AssumeRole"
      }
    ]
  })
}

#############################################
# Attach SSM
#############################################

resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.gproxy.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

#############################################
# Allow reading Load Balancers
#############################################

resource "aws_iam_role_policy_attachment" "elb" {
  role       = aws_iam_role.gproxy.name
  policy_arn = "arn:aws:iam::aws:policy/ElasticLoadBalancingReadOnly"
}

#############################################
# Optional EC2 Read permissions
#############################################

resource "aws_iam_role_policy_attachment" "ec2" {
  role       = aws_iam_role.gproxy.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ReadOnlyAccess"
}

#############################################
# Allow EC2 to download artifacts
#############################################

resource "aws_iam_role_policy" "artifact_download" {
  name = "gproxy-artifact-download"
  role = aws_iam_role.gproxy.id

  policy = jsonencode({
    Version = "2012-10-17"

    Statement = [
      {
        Sid    = "ReadArtifacts"
        Effect = "Allow"

        Action = [
          "s3:GetObject"
        ]

        Resource = [
          "arn:aws:s3:::gproxy-tfstate/artifacts/*"
        ]
      }
    ]
  })
}

#############################################
# Instance Profile
#############################################

resource "aws_iam_instance_profile" "gproxy" {
  name = "gproxy-profile"
  role = aws_iam_role.gproxy.name
}

#############################################
# EC2 Instance
#############################################

resource "aws_instance" "app" {

  ami = data.aws_ami.ubuntu.id

  instance_type = "t3.micro"

  subnet_id = data.aws_subnets.default.ids[0]

  vpc_security_group_ids = [
    aws_security_group.gproxy.id
  ]

  iam_instance_profile = aws_iam_instance_profile.gproxy.name

  user_data = <<-EOF
    #!/bin/bash
    set -eux

    mkdir -p /opt/gproxy

    systemctl enable amazon-ssm-agent || true
    systemctl restart amazon-ssm-agent || true
  EOF

  tags = {
    Name        = "gproxy"
    Project     = "gproxy"
    Environment = "dev"
  }
}

#############################################
# Elastic IP
#############################################

resource "aws_eip" "app" {
  domain = "vpc"
}

resource "aws_eip_association" "app" {
  allocation_id = aws_eip.app.id
  instance_id   = aws_instance.app.id
}

#############################################
# Route53 Zone
#############################################

data "aws_route53_zone" "zone" {
  name = "grubzo.food."
}

#############################################
# Wildcard DNS
#############################################

resource "aws_route53_record" "wildcard" {
  zone_id = data.aws_route53_zone.zone.zone_id
  name = "*.grubzo.food"
  type = "A"
  ttl = 60

  records = [
    aws_eip.app.public_ip
  ]

  allow_overwrite = true
}

#############################################
# Outputs
#############################################

output "instance_id" {
  value = aws_instance.app.id
}

output "instance_ip" {
  value = aws_eip.app.public_ip
}
