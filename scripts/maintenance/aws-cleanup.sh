#!/bin/sh
#
# Cleanup AWS resources.  By default, this script removes all
# resources with an expirationDate tag who's value is in the past:
#
#   $ aws-cleanup.sh
#
# You can also delete resources associated with a specific cluster
# (the tectonicClusterID tag) by setting the CLUSTER_ID environment
# variable:
#
#   $ CLUSTER_ID=9ef6630b-c3ae-cab1-64f4-dec66e428689 aws-cleanup.sh

JOBS="${JOBS:-1}"

function queue() {
	LIVE="$(jobs | wc -l)"
	while test "${LIVE}" -ge "${JOBS}"
	do
		sleep 1
		LIVE="$(jobs | grep -v Done | wc -l)"
	done
	echo "${@}"
	"${@}" &
}

delete_by_arn() {
	# https://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html
	ARN="${1}" &&
	SCHEME=$(echo "${ARN}" | cut -d: -f1) &&
	PARTITION=$(echo "${ARN}" | cut -d: -f2) &&
	SERVICE=$(echo "${ARN}" | cut -d: -f3) &&
	REGION=$(echo "${ARN}" | cut -d: -f4) &&
	#ACCOUNT=$(echo "${ARN}" | cut -d: -f5) &&  # commented-out until we need it
	RESOURCE=$(echo "${ARN}" | cut -d: -f6-) &&

	if test "${SCHEME}" != arn
	then
		echo "unrecognized scheme: '${SCHEME}'" >&2
		return 1
	fi

	if test "${PARTITION}" != aws
	then
		echo "unrecognized partition: '${PARTITION}'" >&2
		return 1
	fi

	if test -n "${REGION}"
	then
		export AWS_DEFAULT_REGION="${REGION}"
	else
		unset AWS_DEFAULT_REGION
	fi

	case "${SERVICE}" in
	ec2)
		delete_ec2 "${ARN}" "${RESOURCE}"
		;;
	elasticloadbalancing)
		delete_elastic_load_balancer "${ARN}" "${RESOURCE}"
		;;
	iam)
		delete_iam "${ARN}" "${RESOURCE}"
		;;
	route53)
		delete_route53 "${ARN}" "${RESOURCE}"
		;;
	s3)
		delete_s3 "${ARN}" "${RESOURCE}"
		;;
	*)
		echo "unrecognized service: '${SERVICE}' (${ARN})" >&2
		return 1
	esac

	return
}

delete_ec2() {
	ARN="${1}"
	RESOURCE="${2}"
	RESOURCE_TYPE=$(echo "${RESOURCE}" | cut -d/ -f1) &&
	RESOURCE_ID=$(echo "${RESOURCE}" | cut -d/ -f2-) &&

	case "${RESOURCE_TYPE}" in
	elastic-ip)
		delete_ec2_elastic_ip "${ARN}" "${RESOURCE_ID}"
		;;
	instance)
		delete_ec2_instance "${ARN}" "${RESOURCE_ID}"
		;;
	internet-gateway)
		delete_ec2_internet_gateway "${ARN}" "${RESOURCE_ID}"
		;;
	natgateway)
		delete_ec2_nat_gateway "${ARN}" "${RESOURCE_ID}"
		;;
	route-table)
		delete_ec2_route_table "${ARN}" "${RESOURCE_ID}"
		;;
	security-group)
		delete_ec2_security_group "${ARN}" "${RESOURCE_ID}"
		;;
	route-table|subnet|volume|vpc)
		delete_ec2_generic "${ARN}" "${RESOURCE_TYPE}" "${RESOURCE_ID}"
		;;
	*)
		echo "unrecognized EC2 resource type: '${RESOURCE_TYPE}' (${ARN})" >&2
		return 1
	esac
}

delete_ec2_elastic_ip() {
	ARN="${1}"
	RESOURCE_ID="${2}"
	echo "deleting EC2 elastic IP ${RESOURCE_ID} (${ARN})" &&
	aws ec2 release-address --allocation-id "${RESOURCE_ID}"
}

delete_ec2_instance() {
	ARN="${1}"
	RESOURCE_ID="${2}"
	echo "deleting EC2 instance ${RESOURCE_ID} (${ARN})" &&
	aws ec2 terminate-instances --instance-ids "${RESOURCE_ID}"
}

delete_ec2_internet_gateway() {
	ARN="${1}"
	RESOURCE_ID="${2}"

	for VPC in $(aws ec2 describe-internet-gateways --internet-gateway-ids "${RESOURCE_ID}" --query 'InternetGateways[].Attachments[].VpcId' --output text)
	do
		echo "deleting EC2 internet-gateway association ${VPC} for ${RESOURCE_ID}" &&
		aws ec2 detach-internet-gateway --internet-gateway-id "${RESOURCE_ID}" --vpc-id "${VPC}"
	done &&

	echo "deleting EC2 internet gateway ${RESOURCE_ID} (${ARN})" &&
	aws ec2 delete-internet-gateway --internet-gateway-id "${RESOURCE_ID}"
}

delete_ec2_nat_gateway() {
	ARN="${1}"
	RESOURCE_ID="${2}"
	echo "deleting EC2 NAT gateway ${RESOURCE_ID} (${ARN})" &&
	aws ec2 delete-nat-gateway --nat-gateway-id "${RESOURCE_ID}"
}

delete_ec2_route_table() {
	ARN="${1}"
	RESOURCE_ID="${2}"
	ROUTE_TABLE="$(AWS_PROFILE=ci aws ec2 describe-route-tables --route-table-ids "${RESOURCE_ID}" --query 'RouteTables[]' --output json)"
	for CIDR in $(echo "${ROUTE_TABLE}" | jq -r '.[].Routes[] | select(.GatewayId != "local") | .DestinationCidrBlock')
	do
		echo "deleting EC2 route ${CIDR} for ${RESOURCE_ID}" &&
		aws ec2 delete-route --route-table-id "${RESOURCE_ID}" --destination-cidr-block "${CIDR}"
	done

	for ASSOCIATION in $(echo "${ROUTE_TABLE}" | jq -r '.[].Associations[] | select(.Main != true) | .RouteTableAssociationId')
	do
		echo "deleting EC2 route-table association ${ASSOCIATION} for ${RESOURCE_ID}" &&
		aws ec2 disassociate-route-table --association-id "${ASSOCIATION}"
	done

	echo "deleting EC2 route table ${RESOURCE_ID} (${ARN})" &&
	aws ec2 delete-route-table --route-table-id "${RESOURCE_ID}"
}

delete_ec2_security_group() {
	ARN="${1}"
	RESOURCE_ID="${2}"
	aws ec2 describe-security-groups --group-id "${RESOURCE_ID}" --query 'SecurityGroups[]' --output json | jq -c '.[]' | while read -r GROUP
	do
		echo "${GROUP}" | jq -c '.IpPermissionsEgress[]' | while read -r EGRESS
		do
			echo "deleting EC2 security group egress ${EGRESS} for ${RESOURCE_ID}" &&
			aws ec2 revoke-security-group-egress --group-id "${RESOURCE_ID}" --ip-permissions "${EGRESS}"
		done

		echo "${GROUP}" | jq -c '.IpPermissions[]' | while read -r INGRESS
		do
			echo "deleting EC2 security group ingress ${INGRESS} for ${RESOURCE_ID}" &&
			aws ec2 revoke-security-group-ingress --group-id "${RESOURCE_ID}" --ip-permissions "${INGRESS}"
		done
	done

	echo "deleting EC2 security group ${RESOURCE_ID} (${ARN})" &&
	aws ec2 delete-security-group --group-id "${RESOURCE_ID}"
}

delete_ec2_generic() {
	ARN="${1}"
	RESOURCE_TYPE="${2}"
	RESOURCE_ID="${3}"
	echo "deleting EC2 $(echo "${RESOURCE_TYPE}" | sed 's/-/ /g') ${RESOURCE_ID} (${ARN})" &&
	aws ec2 "delete-${RESOURCE_TYPE}" "--${RESOURCE_TYPE}-id" "${RESOURCE_ID}"
}

delete_elastic_load_balancer() {
	# https://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html
	#
	# Network load balancer:
	# arn:aws:elasticloadbalancing:region:account-id:loadbalancer/net/load-balancer-name/load-balancer-id
	# arn:aws:elasticloadbalancing:region:account-id:listener/net/load-balancer-name/load-balancer-id/listener-id
	# arn:aws:elasticloadbalancing:region:account-id:listener-rule/net/load-balancer-name/load-balancer-id/listener-id/rule-id
	# arn:aws:elasticloadbalancing:region:account-id:targetgroup/target-group-name/target-group-id
	#
	# Classic load balancer:
	# arn:aws:elasticloadbalancing:region:account-id:loadbalancer/name

	ARN="${1}"
	RESOURCE="${2}"
	RESOURCE_TYPE=$(echo "${RESOURCE}" | cut -d/ -f1) &&
	RESOURCE_ID=$(echo "${RESOURCE}" | cut -d/ -f2-) &&

	case "${RESOURCE_TYPE}" in
	loadbalancer)
		RESOURCE_SUBTYPE=$(echo "${RESOURCE_ID}" | cut -d/ -f1) &&
		case "${RESOURCE_SUBTYPE}" in
		'')
			echo "deleting classic load balancer ${RESOURCE_ID} (${ARN})" &&
			aws elb delete-load-balancer --load-balancer-name "${RESOURCE_ID}"
			;;
		net)
			echo "deleting network load balancer ${RESOURCE_ID} (${ARN})" &&
			aws elbv2 delete-load-balancer --load-balancer-arn "${ARN}"
			;;
		*)
			echo "unrecognized load balancer sub-type: '${RESOURCE_SUBTYPE}' (${ARN})" >&2
			return 1
		esac
		;;
	targetgroup)
		echo "deleting load balancer target group ${RESOURCE_ID} (${ARN})" &&
		aws elbv2 delete-target-group --target-group-arn "${ARN}"
		;;
	*)
		echo "unrecognized load balancer type: '${RESOURCE_TYPE}' (${ARN})" >&2
		return 1
	esac
}

delete_iam() {
	ARN="${1}"
	RESOURCE="${2}"
	RESOURCE_TYPE=$(echo "${RESOURCE}" | cut -d/ -f1) &&
	RESOURCE_ID=$(echo "${RESOURCE}" | cut -d/ -f2-) &&

	if test "${RESOURCE_TYPE}" != "role"
	then
		echo "unrecognized IAM type: '${RESOURCE_TYPE}' (${ARN})" >&2
		return 1
	fi

	for POLICY in $(aws iam list-role-policies --role-name "${RESOURCE_ID}" --query 'PolicyNames[]' --output text)
	do
		echo "deleting IAM role policy ${POLICY} from ${RESOURCE_ID}" &&
		aws iam delete-role-policy --role-name "${RESOURCE_ID}" --policy-name "${POLICY}"
	done

	# Apparently there is no way to delete these using .InstanceProfileId?
	for PROFILE in $(aws iam list-instance-profiles-for-role --role "${RESOURCE_ID}" --query 'InstanceProfiles[].InstanceProfileName' --output text)
	do
		echo "removing role ${RESOURCE_ID} from instance-profile ${PROFILE}" &&
		aws iam remove-role-from-instance-profile --instance-profile-name "${PROFILE}" --role-name "${RESOURCE_ID}"
	done

	echo "deleting IAM role ${RESOURCE_ID} (${ARN})" &&
	aws iam delete-role --role-name "${RESOURCE_ID}"
}

delete_route53() {
	ARN="${1}"
	RESOURCE="${2}"
	RESOURCE_TYPE=$(echo "${RESOURCE}" | cut -d/ -f1) &&
	RESOURCE_ID=$(echo "${RESOURCE}" | cut -d/ -f2-) &&

	if test "${RESOURCE_TYPE}" != "hostedzone"
	then
		echo "unrecognized Route 53 type: '${RESOURCE_TYPE}' (${ARN})" >&2
		return 1
	fi

	aws route53 list-resource-record-sets --hosted-zone-id "${RESOURCE_ID}" --query 'ResourceRecordSets' --output json | jq -c '.[]' | while read -r RECORD
	do
		echo "deleting Route 53 record set ${RECORD}" &&
		aws route53 change-resource-record-sets --hosted-zone-id "${RESOURCE_ID}" --change-batch "
			{
				\"Changes\": [
					{
						\"Action\": \"DELETE\",
						\"ResourceRecordSet\": ${RECORD}
					}
				]
			}"
	done

	echo "deleting Route 53 hosted zone ${RESOURCE_ID} (${ARN})" &&
	aws route53 delete-hosted-zone --id "${RESOURCE_ID}"
}

delete_s3() {
	ARN="${1}"
	BUCKET="${2}"
	echo "deleting S3 bucket ${BUCKET} (${ARN})" &&
	aws s3 rm "s3://${BUCKET}" --recursive &&
	aws s3api delete-bucket --bucket "${BUCKET}"
}

if test -n "${CLUSTER_ID}"
then
	TAG_FILTER="Key == 'tectonicClusterID' && Value == '${CLUSTER_ID}'"
	URL_FILTER="%7B%22resourceTypes%22:%22all%22,%22tagFilters%22:%5B%7B%22key%22:%22tectonicClusterID%22,%22values%22:%5B%22${CLUSTER_ID}%22%5D%7D%5D%7D" 
else
	NOW="$(date --utc '+%Y-%m-%dT%H:%M')"
	TAG_FILTER="Key == 'expirationDate' && Value < '${NOW}'"
	URL_FILTER="%7B%22resourceTypes%22:%22all%22,%22tagFilters%22:%5B%7B%22key%22:%22expirationDate%22%7D%5D%7D"  # FIXME: < filter for dates?
fi

# FIXME: update to use 'aws resource-groups search-resources'?
# https://docs.aws.amazon.com/cli/latest/reference/resource-groups/search-resources.html
ARNS="$(aws resourcegroupstaggingapi get-resources --query "ResourceTagMappingList[?Tags[? ${TAG_FILTER}]].ResourceARN" --output text)"
ec=0
for ARN in ${ARNS}
do
	queue delete_by_arn "${ARN}" ||
	ec=1
done

ec=1  # FIXME: collect the exit status of queued commands
if ! wait
then
	ec=1
fi

if test "${ec}" -eq 1
then
	echo "For links to the remaining resources, see https://resources.console.aws.amazon.com/r/group#sharedgroup=${URL_FILTER}" >&2
	echo "Deleting VPCs seems to be the most reliable way to break dependency blocks." >&2
fi

exit "${ec}"
