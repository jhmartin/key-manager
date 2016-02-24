package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/aws/credentials"
  "github.com/aws/aws-sdk-go/aws/ec2metadata"
  "github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
)

type awsHostContext struct {
  Creds   *credentials.Credentials
  Id      string
  Region  string
}

func main() {
	// currently not using any of the client flags
	_ = flag.String("u", "", "The user account")
	_ = flag.String("k", "", "The public key proffered by the client")
	_ = flag.String("t", "", "The key encryption type")
	_ = flag.String("f", "", "The fingerprint of the public key")
	groupTag := flag.String("group_tag", "access-groups", "The instance tag with csv ssh sccess groups in it")
	s3Bucket := flag.String("s3_bucket", "keys", "The bucket where the access group public keys are stored")
	s3Region := flag.String("s3_region", "eu-west-1", "The region in which the bucket is located")
	flag.Parse()

	// get aws host context
	hctx, err := getAwsHostContext()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// get the list of access groups the intance belongs to
	accessGroups, err := getInstanceAccessGroups(hctx, *groupTag)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// get the authorized_keys files for those instances
	authorizedKeys, err := getAccessKeys(hctx, *s3Bucket, *s3Region, accessGroups)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// print all authorized_keys
	for _, k := range authorizedKeys {
		fmt.Print(k)
	}
}


func getAwsHostContext() (*awsHostContext, error) {
  //setup
  hctx := new(awsHostContext)
  cfg := aws.NewConfig()
  svc := ec2metadata.New(session.New(), cfg)
  p := &ec2rolecreds.EC2RoleProvider{
		Client: svc,
	}
  //get creds
  hctx.Creds = credentials.NewCredentials(p)
  //get instance id
  id, err := svc.GetMetadata("instance-id")
  if err != nil {
    return nil, err
  }
  hctx.Id = id
  //get region
  region, err := svc.Region()
  if err != nil {
    return nil, err
  }
  hctx.Region = region
  return hctx, nil
}


func getInstanceAccessGroups(hctx *awsHostContext, tag string) ([]string, error) {
	// setup
	svc := ec2.New(session.New(aws.NewConfig().WithCredentials(hctx.Creds).WithRegion(hctx.Region)))
	params := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("resource-id"),
				Values: []*string{ aws.String(hctx.Id) },
			},
			{
				Name: aws.String("key"),
				Values: []*string{ aws.String(tag) },
			},
		},
	}
	// fetch tags
	resp, err := svc.DescribeTags(params)
	if err != nil {
		return []string{}, err
	}

	// parse and return response
	var out []string
	for _, s := range strings.Split(*resp.Tags[0].Value, ",") {
		out = append(out, strings.TrimSpace(s))
	}
	return out, nil
}


func getAccessKeys(hctx *awsHostContext, s3Bucket, s3Region string, accessGroups []string) ([]string, error) {
	// setup
	svc := s3.New(session.New(aws.NewConfig().WithCredentials(hctx.Creds).WithRegion(s3Region)))

	var out []string
	// fetch authorized_keys files
	for _, group := range accessGroups {
		params := &s3.GetObjectInput{
			Bucket:  aws.String(s3Bucket),
			Key:     aws.String(group + "/authorized_keys"),  // Required
		}
		resp, err := svc.GetObject(params)
		if err != nil {
			continue
		}
		if b, err := ioutil.ReadAll(resp.Body); err == nil {
    	out = append(out, string(b))
		}
	}

	return out, nil
}
