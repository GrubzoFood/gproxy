package alb

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cfg "gproxy/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"go.uber.org/zap"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type AWSConfig struct {
	cfg    aws.Config
	rt     *cfg.RouteTable
	logger *zap.Logger
}

func InitAWSConfig(
	rt *cfg.RouteTable,
	logger *zap.Logger,
	isDev bool,
) (*AWSConfig, error) {
	ops := []func(*config.LoadOptions) error{
		config.WithRegion("ap-south-2"),
	}
	if isDev {
		ops = append(ops,
			config.WithSharedCredentialsFiles([]string{"/home/rahul/.aws/credentials"}),
		)
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(), ops...)
	if err != nil {
		return nil, fmt.Errorf("failed aws config init: %v", err)
	}
	return &AWSConfig{cfg, rt, logger}, nil
}

func (cfg *AWSConfig) register() {
	elbv2Client := elbv2.NewFromConfig(cfg.cfg)
	albs, err := elbv2Client.DescribeLoadBalancers(context.Background(), &elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		cfg.logger.Error("failed fetching loadbalancers from aws", zap.Error(err))
		return
	}

	type ALB struct {
		awsLoadBalancerRef types.LoadBalancer

		arn     string
		dns     string
		cluster string
	}

	albMap := map[string]ALB{}
	arns := []string{}
	for _, v := range albs.LoadBalancers {
		arns = append(arns, *v.LoadBalancerArn)
		albMap[*v.LoadBalancerArn] = ALB{
			awsLoadBalancerRef: v,

			arn: *v.LoadBalancerArn,
			dns: *v.DNSName,
		}
	}

	describeTags, err := elbv2Client.DescribeTags(context.Background(), &elbv2.DescribeTagsInput{ResourceArns: arns})
	if err != nil {
		cfg.logger.Error("failed describing the alb tags", zap.Error(err))
		return
	}
	for _, desc := range describeTags.TagDescriptions {
		for _, v := range desc.Tags {
			if *v.Key == "elbv2.k8s.aws/cluster" {
				alb := albMap[*desc.ResourceArn]
				alb.cluster = *v.Value
				albMap[*desc.ResourceArn] = alb

				break
			}
		}
	}

	getSubDomains := func(dns, subdomain string) ([]string, error) {

		host := fmt.Sprintf("%s.grubzo.food", subdomain)
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName: host,
			},
		}
		client := &http.Client{
			Transport: transport,
		}
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:443/api/v1/tenants", dns), nil)
		if err != nil {
			return nil, err
		}

		req.Host = host
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status: %s", res.Status)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		tenants := strings.Split(strings.TrimSpace(string(body)), ",")
		return tenants, nil
	}

	for _, v := range albMap {
		if v.cluster == "ops" {
			if err := cfg.rt.RegisterRoute("argocd", fmt.Sprintf("http://%s:%d", v.dns, 443)); err != nil {
				cfg.logger.Error(fmt.Sprintf("failed registering subdomain: ops in instance: %s", v.cluster), zap.Error(err))
			}
		} else {
			cfg.rt.RegisterRoute(v.cluster, fmt.Sprintf("http://%s:%d", v.dns, 443))
			subDomains, err := getSubDomains(v.dns, v.cluster)
			if err != nil {
				cfg.logger.Error(fmt.Sprintf("failed fetching subdomains from instance with error: %s ", v.cluster), zap.Error(err))
				continue
			}
			for _, subDomain := range subDomains {
				cfg.logger.Info(fmt.Sprintf("registred subdomain: %s in instance: %s\n", subDomain, v.cluster))
				if err := cfg.rt.RegisterRoute(subDomain, fmt.Sprintf("http://%s:%d", v.dns, 443)); err != nil {
					cfg.logger.Error(fmt.Sprintf("failed registering subdomain: %s in instance: %s, with error: %s ", subDomain, v.cluster), zap.Error(err))
				}
			}
		}
	}
}

func (cfg *AWSConfig) Register(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	cfg.register()

	for {
		select {
		case <-ticker.C:
			cfg.register()
		case <-ctx.Done():
			return
		}
	}
}
