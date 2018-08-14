package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

  "bytes"
	"flag"
	"fmt"
  "io"
  "io/ioutil"
	"log"
	"os"
	"time"
)

var awsRole string
var awsBucket string
var awsKey string
var awsExpiration string
var awsFile string
var awsTransfer bool

func init() {
	flag.StringVar(&awsRole, "role", "", "iam role to assume")
	flag.StringVar(&awsFile, "file", "", "path to file for a PUT request")
	flag.StringVar(&awsFile, "f", "", "path to file for a PUT request (shorthand)")
	flag.StringVar(&awsExpiration, "expiration", "15m", "duration before presigned url expiration")
	flag.StringVar(&awsExpiration, "e", "15m", "duration before presigned url expiration (shorthand)")
	flag.BoolVar(&awsTransfer, "transfer", false, "perform the actual request instead of presigning a url")
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s: [OPTIONS] bucket key\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
  var f io.ReadSeeker

	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 2 {
    args := flag.Args()
		awsBucket = args[0]
		awsKey = args[1]
	}

	if awsBucket == "" || awsKey == "" {
		printErrorUsage("Bucket and key are required fields.")
	}

	exp, err := time.ParseDuration(awsExpiration)
	if err != nil {
		printErrorUsage(fmt.Sprintf("Unable to parse expiration: %s", exp))
	}

	c := newClient(awsRole, awsTransfer)

  if isPipe(os.Stdin) && (awsFile == "-" || awsFile == "") {
    if buf, err := ioutil.ReadAll(os.Stdin); err == nil {
      if len(buf) > 0 {
        f = bytes.NewReader(buf)
      }
    }
  }

  if !isPipe(os.Stdin) {
    if awsFile != "" {

      file, err := os.Open(awsFile)
      defer file.Close()

      if err != nil {
        log.Fatal(err)
      }

      f = file
    }
  }

  var params interface{}

  if f != nil {
    params = &s3.PutObjectInput{Bucket: &awsBucket, Key: &awsKey, Body: f}
  } else {
    params = &s3.GetObjectInput{Bucket: &awsBucket, Key: &awsKey}
  }

  if awsTransfer {
    if f == nil {
      downloader := s3manager.NewDownloaderWithClient(c)
      buf := aws.NewWriteAtBuffer([]byte{})
      if _, err := downloader.Download(buf, params.(*s3.GetObjectInput)); err != nil {
        log.Fatal(err)
      } else {
        _, err := io.Copy(os.Stdout, bytes.NewReader(buf.Bytes()))
        if err != nil {
          log.Fatal(err)
        }
      }
    } else {
      uploader := s3manager.NewUploaderWithClient(c)
      params := &s3manager.UploadInput{Bucket: &awsBucket, Key: &awsKey, Body: f}
      res, err := uploader.Upload(params)
      if err != nil {
        log.Fatal(err)
      }
      fmt.Print(res.Location)
    }
  } else {
    printOrFatal(c.presign(exp, params))
  }
}

func printOrFatal(s string, err error) {
	if err == nil {
		fmt.Print(s)
	} else {
		log.Fatalf("%+v", err)
	}
}

func isPipe(f *os.File) bool {
  if f == nil {
    f = os.Stdin
  }
  if fi, err := f.Stat(); err == nil {
    if (fi.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
      return true
    }
  }
  return false
}

type client struct {
	*s3.S3
}

func printErrorUsage(s string) {
	fmt.Fprintf(flag.CommandLine.Output(), "%s\n\n", s)
	flag.Usage()
	os.Exit(1)
}

func newClient(r string, t bool) *client {
	conf := aws.NewConfig()
	sess := session.Must(session.NewSession())

	if r != "" {
		creds := stscreds.NewCredentials(sess, r)
		conf = conf.WithCredentials(creds)
	}

	return &client{s3.New(sess, conf)}
}

func (c *client) presign(exp time.Duration, p interface{}) (string, error) {
  var req *request.Request
  switch v := p.(type) {
  case *s3.GetObjectInput:
    req, _ = c.GetObjectRequest(v)
  case *s3.PutObjectInput:
    req, _ = c.PutObjectRequest(v)
  default:
    return "", fmt.Errorf("unknown type to presign")
  }

  return req.Presign(exp)
}
