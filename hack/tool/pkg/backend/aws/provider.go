/*
Copyright 2022 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"tool/pkg/terraform"

	"github.com/hashicorp/terraform-exec/tfexec"
)

const (
	ArgRegion = "region"

	EnvAWSAccessKeyID     = "AWS_ACCESS_KEY_ID"
	EnvAWSSecretAccessKey = "AWS_SECRET_ACCESS_KEY"
	EnvAWSSessionToken    = "AWS_SESSION_TOKEN"

	k0sConfigYamlName = "k0s_cluster"
)

type Provider struct {
}

func kv(key string, value any) string {
	return fmt.Sprintf("%s=%v", key, value)
}

func (p *Provider) Init(ctx context.Context) error {
	// Expect a handful of environment variables set which will provide
	// the access to AWS
	for _, key := range []string{EnvAWSAccessKeyID, EnvAWSSecretAccessKey, EnvAWSSessionToken} {
		if _, found := os.LookupEnv(key); !found {
			return fmt.Errorf("missing required AWS environment variable '%s'", key)
		}
	}

	return nil
}

// vpcinfra create

var vpcInfraPath = path.Join("aws", "commands", "vpcinfra")

func (p *Provider) VpcInfraCreate(ctx context.Context, name string, region string, cidr string) error {
	log.Printf("Creating VPC infrastructure")

	if err := terraform.Apply(ctx, vpcInfraPath,
		tfexec.Var(kv("region", region)),
		tfexec.Var(kv("name", name)),
		tfexec.Var(kv("cidr", cidr)),
	); err != nil {
		return err
	}

	vals, err := terraform.Output(ctx, vpcInfraPath)
	if err != nil {
		return err
	}

	value, found := vals["vpc_id"]
	if !found {
		return fmt.Errorf("value named '%s' not found", "vpc_id")
	}

	var data string
	if err := json.Unmarshal(value.Value, &data); err != nil {
		fmt.Printf("!@#!@# ERROR: %v\n", err)
		return err
	}

	fmt.Println(strings.TrimSpace(data))

	return nil
}

func (p *Provider) VpcInfraDestroy(ctx context.Context, name string, region string, cidr string) error {
	log.Printf("Destroying VPC infrastructure")

	return terraform.Destroy(ctx, vpcInfraPath,
		tfexec.Var(kv("region", region)),
		tfexec.Var(kv("name", name)),
		tfexec.Var(kv("cidr", cidr)),
	)
}

// ha create

var haPath = path.Join("aws", "commands", "ha")

func (p *Provider) ClusterHACreate(ctx context.Context, clusterName string, k0sBinary string, k0sBinaryUpdate string, k0sVersion string, k0sUpdateVersion string, k0sAirgapBundle string, k0sAirgapBundleConfig string, k0sAirgapBundleUpdate string, controllers int, workers int, region string) error {
	log.Printf("Creating k0s HA cluster")

	return terraform.Apply(ctx, haPath,
		tfexec.Var(kv("name", clusterName)),
		tfexec.Var(kv("controllers", controllers)),
		tfexec.Var(kv("workers", workers)),
		tfexec.Var(kv("region", region)),
		tfexec.Var(kv("k0s_binary", k0sBinary)),
		tfexec.Var(kv("k0s_version", k0sVersion)),
		tfexec.Var(kv("k0s_airgap_bundle", k0sAirgapBundle)),
		tfexec.Var(kv("k0s_airgap_bundle_config", k0sAirgapBundleConfig)),
		tfexec.Var(kv("k0s_update_binary", k0sBinaryUpdate)),
		tfexec.Var(kv("k0s_update_airgap_bundle", k0sAirgapBundleUpdate)),
		tfexec.Var(kv("k0s_update_version", k0sUpdateVersion)),
	)
}

func (p *Provider) ClusterHADestroy(ctx context.Context, clusterName string, k0sBinary string, k0sBinaryUpdate string, k0sVersion string, k0sUpdateVersion string, k0sAirgapBundle string, k0sAirgapBundleConfig string, k0sAirgapBundleUpdate string, controllers int, workers int, region string) error {
	log.Printf("Destroying k0s HA cluster")

	return terraform.Destroy(ctx, haPath,
		tfexec.Var(kv("name", clusterName)),
		tfexec.Var(kv("controllers", controllers)),
		tfexec.Var(kv("workers", workers)),
		tfexec.Var(kv("region", region)),
		tfexec.Var(kv("k0s_binary", k0sBinary)),
		tfexec.Var(kv("k0s_version", k0sVersion)),
		tfexec.Var(kv("k0s_airgap_bundle", k0sAirgapBundle)),
		tfexec.Var(kv("k0s_airgap_bundle_config", k0sAirgapBundleConfig)),
		tfexec.Var(kv("k0s_update_binary", k0sBinaryUpdate)),
		tfexec.Var(kv("k0s_update_airgap_bundle", k0sAirgapBundleUpdate)),
		tfexec.Var(kv("k0s_update_version", k0sUpdateVersion)),
	)
}

func (p *Provider) ClusterHAClusterConfig(ctx context.Context) (string, error) {
	return clusterConfig(ctx, haPath)
}

// havpc create

var haVpcPath = path.Join("aws", "commands", "havpc")

func (p *Provider) ClusterHAVpcCreate(ctx context.Context, vpcId string, subnetIdx int, clusterName string, k0sBinary string, k0sBinaryUpdate string, k0sVersion string, k0sUpdateVersion string, k0sAirgapBundle string, k0sAirgapBundleConfig string, k0sAirgapBundleUpdate string, controllers int, workers int, region string) error {
	log.Printf("Creating k0s HA cluster")

	return terraform.Apply(ctx, haVpcPath,
		tfexec.Var(kv("vpc_id", vpcId)),
		tfexec.Var(kv("subnet_idx", subnetIdx)),
		tfexec.Var(kv("name", clusterName)),
		tfexec.Var(kv("controllers", controllers)),
		tfexec.Var(kv("workers", workers)),
		tfexec.Var(kv("region", region)),
		tfexec.Var(kv("k0s_binary", k0sBinary)),
		tfexec.Var(kv("k0s_version", k0sVersion)),
		tfexec.Var(kv("k0s_airgap_bundle", k0sAirgapBundle)),
		tfexec.Var(kv("k0s_airgap_bundle_config", k0sAirgapBundleConfig)),
		tfexec.Var(kv("k0s_update_binary", k0sBinaryUpdate)),
		tfexec.Var(kv("k0s_update_airgap_bundle", k0sAirgapBundleUpdate)),
		tfexec.Var(kv("k0s_update_version", k0sUpdateVersion)),
	)
}

func (p *Provider) ClusterHAVpcDestroy(ctx context.Context, vpcId string, subnetIdx int, clusterName string, k0sBinary string, k0sBinaryUpdate string, k0sVersion string, k0sUpdateVersion string, k0sAirgapBundle string, k0sAirgapBundleConfig string, k0sAirgapBundleUpdate string, controllers int, workers int, region string) error {
	log.Printf("Destroying k0s HA cluster")

	return terraform.Destroy(ctx, haVpcPath,
		tfexec.Var(kv("vpc_id", vpcId)),
		tfexec.Var(kv("subnet_idx", subnetIdx)),
		tfexec.Var(kv("name", clusterName)),
		tfexec.Var(kv("controllers", controllers)),
		tfexec.Var(kv("workers", workers)),
		tfexec.Var(kv("region", region)),
		tfexec.Var(kv("k0s_binary", k0sBinary)),
		tfexec.Var(kv("k0s_version", k0sVersion)),
		tfexec.Var(kv("k0s_airgap_bundle", k0sAirgapBundle)),
		tfexec.Var(kv("k0s_airgap_bundle_config", k0sAirgapBundleConfig)),
		tfexec.Var(kv("k0s_update_binary", k0sBinaryUpdate)),
		tfexec.Var(kv("k0s_update_airgap_bundle", k0sAirgapBundleUpdate)),
		tfexec.Var(kv("k0s_update_version", k0sUpdateVersion)),
	)
}

func (p *Provider) ClusterHAVpcClusterConfig(ctx context.Context) (string, error) {
	return clusterConfig(ctx, haVpcPath)
}

func clusterConfig(ctx context.Context, path string) (string, error) {
	log.Printf("Retrieving generated cluster config")

	vals, err := terraform.Output(ctx, haPath)
	if err != nil {
		return "", err
	}

	value, found := vals[k0sConfigYamlName]
	if !found {
		return "", fmt.Errorf("value named '%s' not found", k0sConfigYamlName)
	}

	var data string
	if err := json.Unmarshal(value.Value, &data); err != nil {
		fmt.Printf("!@#!@# ERROR: %v\n", err)
		return "", err
	}

	return strings.TrimSpace(data), nil
}
