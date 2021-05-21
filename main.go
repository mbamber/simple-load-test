package main

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/xfxdev/xlog"
)

const (
	maxRequestsPerThread = 20
)

var (
	debug             bool
	headers           map[string]string
	okCodes           []int
	requestsPerSecond int
	timeoutSeconds    int
)

var rootCmd = &cobra.Command{
	Use:   "slt",
	Short: "Run a simple load test",
	Long:  "Run a simple load test against a given endpoint",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("expected 1 URL")
		}

		_, err := url.Parse(args[0])
		if err != nil {
			return errors.New("unable to parse argument to a valid URL")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		logLevel := xlog.InfoLevel
		if debug {
			logLevel = xlog.DebugLevel
		}
		logger := xlog.New(logLevel, os.Stdout, "%L %l")
		return sendRequests(logger, args[0], headers, okCodes, requestsPerSecond, timeoutSeconds)
	},
}

func sendRequests(logger *xlog.Logger, url string, headers map[string]string, okCodes []int, rps, timeout int) error {
	logger.Infof("Starting load test to %s", url)
	logger.Infof("Sending %d requests per second", rps)

	h := http.DefaultClient
	h.Timeout = time.Second * time.Duration(timeout)

	var okCount, errCount int
	var responses = make(chan bool)
	var fatal = make(chan error)

	// Thread to count the responses
	go func(responses chan bool) {
		for r := range responses {
			if r {
				okCount++
			} else {
				errCount++
			}
		}
	}(responses)

	// Thread to print data about the requests
	go func(logger *xlog.Logger) {
		for {
			logger.Infof("Sent %d requests, %d ok, %d failures", okCount+errCount, okCount, errCount)
			time.Sleep(5 * time.Second)
		}
	}(logger)

	// Build the request for re-use
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		xlog.Error(err)
		return err
	}

	for key, val := range headers {
		req.Header.Add(key, val)
	}

	numThreads := (rps / maxRequestsPerThread) + 1
	logger.Debugf("Using %d threads, with maximum %d requests per thread (maximum %d per second)", numThreads, maxRequestsPerThread, numThreads*maxRequestsPerThread)

	// Thread to make requests
	timer := time.NewTimer(time.Second)
	go func(logger *xlog.Logger, timer *time.Timer) {
		for {
			<-timer.C // wait for the timer to fire

			// Send each request in its own thread
			for i := 0; i < numThreads; i++ {
				reqsForThisThread := rps % ((i + 1) * maxRequestsPerThread)
				if reqsForThisThread > maxRequestsPerThread {
					reqsForThisThread = maxRequestsPerThread
				}

				go sendNRequests(logger, h, req, okCodes, responses, fatal, reqsForThisThread)
			}
			timer.Reset(time.Second) // Reset the timer so it fires again
		}
	}(logger, timer)

	e := <-fatal
	logger.Fatal(e)
	timer.Stop() // Stop the timer
	return e
}

func sendNRequests(logger *xlog.Logger, h *http.Client, req *http.Request, okCodes []int, responses chan bool, fatal chan error, n int) {
	logger.Debugf("Sending %d requests in thread", n)
	for i := 0; i < n; i++ {
		sendRequest(logger, h, req, okCodes, responses, fatal)
	}
}

// sendRequest sends a single request
func sendRequest(logger *xlog.Logger, h *http.Client, req *http.Request, okCodes []int, responses chan bool, fatal chan error) {
	resp, err := h.Do(req)
	if err != nil {
		fatal <- err
	}
	resp.Body.Close()

	for _, c := range okCodes {
		if c == resp.StatusCode {
			responses <- true
			return
		}
	}
	responses <- false
	logger.Debugf("Request failed with code %q", resp.Status)
}

func main() {
	rootCmd.Execute()
}

func init() {
	pflag.BoolVarP(&debug, "debug", "v", false, "enable verbose logging")
	pflag.IntVarP(&requestsPerSecond, "requests-per-second", "r", 1, "approximate number of requests to make per second")
	pflag.IntVarP(&timeoutSeconds, "timeout-seconds", "t", 10, "maximum number of seconds for each request to complete before it timesout")
	pflag.StringToStringVarP(&headers, "headers", "e", map[string]string{}, "additional headers to include in each request")
	pflag.IntSliceVarP(&okCodes, "ok-codes", "o", []int{200}, "list of status codes to consider as OK")
}
