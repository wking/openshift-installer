package machines

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/pricing"
)

var (
	regionMap = map[string]string{
		"US East (N. Virginia)":      "us-east-1",
		"US East (Ohio)":             "us-east-2",
		"US West (N. California)":    "us-west-1",
		"US West (Oregon)":           "us-west-2",
		"Asia Pacific (Mumbai)":      "ap-south-1",
		"Asia Pacific (Tokyo)":       "ap-northeast-1",
		"Asia Pacific (Seoul)":       "ap-northeast-2",
		"Asia Pacific (Osaka-Local)": "ap-northeast-3",
		"Asia Pacific (Singapore)":   "ap-southeast-1",
		"Asia Pacific (Sydney)":      "ap-southeast-2",
		"Canada (Central)":           "ca-central-1",
		"EU (Frankfurt)":             "eu-central-1",
		"EU (Ireland)":               "eu-west-1",
		"EU (London)":                "eu-west-2",
		"EU (Paris)":                 "eu-west-3",
		"EU (Stockholm)":             "eu-north-1",
		"South America (Sao Paulo)":  "sa-east-1",
		"AWS GovCloud (US-East)":     "us-gov-east-1",
		"AWS GovCloud (US)":          "us-gov-west-1",
	}

	// ordered list of prefered instanceClasses
	preferredInstanceClass = []string{"m4", "m5"}
)

type InstanceClasses struct {
	Items map[string]struct{}
}

func (s *InstanceClasses) Add(t string) *InstanceClasses {
	if s.Items == nil {
		s.Items = make(map[string]struct{})
	}
	_, ok := s.Items[t]
	if !ok {
		s.Items[t] = struct{}{}
	}
	return s
}

func (s *InstanceClasses) Has(item string) bool {
	_, ok := s.Items[item]
	return ok
}

func (s *InstanceClasses) Preferred() string {
	for _, class := range preferredInstanceClass {
		if s.Has(class) {
			return class
		}
	}
	return "UNKNOWN"
}

func TestDefaultRegionInstanceTypeMap(t *testing.T) {
	svc := pricing.New(session.New(&aws.Config{
		Region: aws.String("us-east-1"),
	}))
	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []*pricing.Filter{
			{
				Field: aws.String("tenancy"),
				Type:  aws.String("TERM_MATCH"),
				Value: aws.String("Shared"),
			},
			{
				Field: aws.String("productFamily"),
				Type:  aws.String("TERM_MATCH"),
				Value: aws.String("Compute Instance"),
			},
		},
	}

	instanceClasses := map[string]*InstanceClasses{}

	handlePage := func(result *pricing.GetProductsOutput, dunno bool) bool {
		for _, priceList := range result.PriceList {
			product := priceList["product"].(map[string]interface{})
			attr := product["attributes"].(map[string]interface{})
			location := attr["location"].(string)
			instanceType := attr["instanceType"].(string)
			instanceClassSlice := strings.Split(instanceType, ".")
			instanceClass := instanceClassSlice[0]
			classes, ok := instanceClasses[location]
			if !ok {
				classes = &InstanceClasses{}
			}
			instanceClasses[location] = classes.Add(instanceClass)
		}
		return true
	}
	err := svc.GetProductsPages(input, handlePage)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case pricing.ErrCodeInternalErrorException:
				t.Error(pricing.ErrCodeInternalErrorException, aerr.Error())
			case pricing.ErrCodeInvalidParameterException:
				t.Error(pricing.ErrCodeInvalidParameterException, aerr.Error())
			case pricing.ErrCodeNotFoundException:
				t.Error(pricing.ErrCodeNotFoundException, aerr.Error())
			case pricing.ErrCodeInvalidNextTokenException:
				t.Error(pricing.ErrCodeInvalidNextTokenException, aerr.Error())
			case pricing.ErrCodeExpiredNextTokenException:
				t.Error(pricing.ErrCodeExpiredNextTokenException, aerr.Error())
			default:
				t.Errorf("Error fetching from AWS: %v", aerr.Error())
			}
		} else {
			t.Errorf("Error fetching from AWS: %v", err.Error())
		}
		return
	}

	preferredInstance := map[string]string{}
	for location, classes := range instanceClasses {
		region, ok := regionMap[location]
		if !ok {
			t.Errorf("Location: %q not found in regionMap", location)
			continue
		}
		class := classes.Preferred()
		if class == "unknown" {
			t.Errorf("Region: %s unable to find default in %v", region, classes.Items)
		}
		preferredInstance[region] = class
	}

	if !reflect.DeepEqual(preferredInstance, defaultMachineClass) {
		t.Errorf("defaultMachineClass: %v is different than preferredInstance: %v", defaultMachineClass, preferredInstance)
	}
}
