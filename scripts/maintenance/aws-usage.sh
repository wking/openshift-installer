#!/usr/bin/sh

if test "${#}" -eq 0
then
	set us-east-1 us-east-2 us-west-1 us-west-2
fi

TOTAL_VPCS=0
TOTAL_EIPS=0
TOTAL_NATS=0
TOTAL_NLBS=0

printf "%s\t%s\t%s\t%s\t%s\n" "region  " "VPCs" "EIPs" "NATs" "NLBs"
for REGION in "${@}"
do
	export AWS_DEFAULT_REGION="${REGION}"
	VPCS=$(aws ec2 describe-vpcs --output text --query 'Vpcs[].VpcId' | wc -w)
	EIPS=$(aws ec2 describe-addresses --output text --query 'Addresses[].AllocationId' | wc -w)
	NATS=$(aws ec2 describe-nat-gateways --output text --query 'NatGateways[].NatGatewayId' | wc -w)
	NLBS=$(aws elbv2 describe-load-balancers --output text --query 'LoadBalancers[].LoadBalancerArn' | wc -w)
	printf "%s\t%s\t%s\t%s\t%s\n" "${REGION}" "${VPCS}" "${EIPS}" "${NATS}" "${NLBS}"
	TOTAL_VPCS=$((TOTAL_VPCS + VPCS))
	TOTAL_EIPS=$((TOTAL_EIPS + EIPS))
	TOTAL_NATS=$((TOTAL_NATS + NATS))
	TOTAL_NLBS=$((TOTAL_NLBS + NLBS))
done
printf "%s\t%s\t%s\t%s\t%s\n" "totals  " "${TOTAL_VPCS}" "${TOTAL_EIPS}" "${TOTAL_NATS}" "${TOTAL_NLBS}"
