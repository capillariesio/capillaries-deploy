package cldaws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/capillariesio/capillaries-deploy/pkg/l"
)

func GetPublicIpAllocation(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, publicIp string) (string, error) {
	if publicIp == "" {
		return "", fmt.Errorf("empty parameter not allowed: publicIp (%s)", publicIp)
	}
	out, err := client.DescribeAddresses(goCtx, &ec2.DescribeAddressesInput{PublicIps: []string{publicIp}})
	lb.AddObject("DescribeAddresses", out)
	if err != nil {
		return "", fmt.Errorf("cannot get public IP %s allocation id: %s", publicIp, err.Error())
	}
	if len(out.Addresses) == 0 {
		return "", nil
	}

	return *out.Addresses[0].AllocationId, nil
}

func GetPublicIpAssoiatedInstance(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, publicIp string) (string, error) {
	if publicIp == "" {
		return "", fmt.Errorf("empty parameter not allowed: publicIp (%s)", publicIp)
	}
	out, err := client.DescribeAddresses(goCtx, &ec2.DescribeAddressesInput{PublicIps: []string{publicIp}})
	//Filters: []types.Filter{{Name: aws.String("public-ip"), Values: []string{publicIp}}}})
	lb.AddObject("DescribeAddresses", out)
	if err != nil {
		return "", fmt.Errorf("cannot check public IP instance id %s:%s", publicIp, err.Error())
	}

	if len(out.Addresses) > 0 && out.Addresses[0].InstanceId != nil {
		return *out.Addresses[0].InstanceId, nil
	}

	return "", nil
}

func AllocateFloatingIp(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, publicIpDesc string) (string, error) {
	if publicIpDesc == "" {
		return "", fmt.Errorf("empty parameter not allowed: publicIpDesc (%s)", publicIpDesc)
	}
	out, err := client.AllocateAddress(goCtx, &ec2.AllocateAddressInput{TagSpecifications: []types.TagSpecification{{
		ResourceType: types.ResourceTypeElasticIp,
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String(publicIpDesc)}}}}})
	lb.AddObject("AllocateAddress", out)
	if err != nil {
		return "", fmt.Errorf("cannot allocate %s IP address:%s", publicIpDesc, err.Error())
	}

	return *out.PublicIp, nil
}

func ReleaseFloatingIp(client *ec2.Client, goCtx context.Context, lb *l.LogBuilder, publicIp string) error {
	if publicIp == "" {
		return fmt.Errorf("empty parameter not allowed: publicIp (%s)", publicIp)
	}

	allocationId, err := GetPublicIpAllocation(client, goCtx, lb, publicIp)
	if err != nil {
		return fmt.Errorf("cannot find IP address %s to delete:%s", publicIp, err.Error())
	}

	if allocationId != "" {
		outDel, err := client.ReleaseAddress(goCtx, &ec2.ReleaseAddressInput{AllocationId: aws.String(allocationId)})
		lb.AddObject("ReleaseAddress", outDel)
		if err != nil {
			return fmt.Errorf("cannot release IP address %s to delete:%s", publicIp, err.Error())
		}
	}
	return nil
}
