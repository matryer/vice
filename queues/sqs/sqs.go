// Package sqs provides a Vice implementation for Amazon Simple Queue Service.
package sqs

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/matryer/vice/v2"
)

// Transport is a vice.Transport for Amazon's SQS
type Transport struct {
	batchSize     int
	batchInterval time.Duration
	wg            *sync.WaitGroup

	sm        sync.Mutex
	sendChans map[string]chan []byte

	rm           sync.Mutex
	receiveChans map[string]chan []byte

	errChan     chan error
	stopchan    chan struct{}
	stopPubChan chan struct{}
	stopSubChan chan struct{}

	NewService func(region string) (sqsiface.SQSAPI, error)
}

// New returns a new transport
// Credentials are automatically sourced using the AWS SDK credential chain,
// for more info see the AWS SDK docs:
// https://godoc.org/github.com/aws/aws-sdk-go#hdr-Configuring_Credentials
func New(batchSize int, batchInterval time.Duration) *Transport {
	if batchSize > 10 {
		batchSize = 10
	}

	if batchInterval == 0 {
		batchInterval = 200 * time.Millisecond
	}

	return &Transport{
		wg:            &sync.WaitGroup{},
		batchSize:     batchSize,
		batchInterval: batchInterval,
		sendChans:     make(map[string]chan []byte),
		receiveChans:  make(map[string]chan []byte),
		errChan:       make(chan error, 10),
		stopchan:      make(chan struct{}),
		stopPubChan:   make(chan struct{}),
		stopSubChan:   make(chan struct{}),

		NewService: func(region string) (sqsiface.SQSAPI, error) {
			awsConfig := aws.NewConfig().WithRegion(region)
			s, err := session.NewSession(awsConfig)
			if err != nil {
				return nil, err
			}
			return sqs.New(s), nil
		},
	}
}

// Receive gets a channel on which to receive messages
// with the specified name. The name is the queue's url
func (t *Transport) Receive(name string) <-chan []byte {
	t.rm.Lock()
	defer t.rm.Unlock()

	ch, ok := t.receiveChans[name]
	if ok {
		return ch
	}

	ch, err := t.makeSubscriber(name)
	if err != nil {
		t.errChan <- &vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	t.receiveChans[name] = ch
	return ch
}

// RegionFromURL parses an sqs url and returns the aws region
func RegionFromURL(url string) string {
	pieces := strings.Split(url, ".")
	if len(pieces) > 2 {
		return pieces[1]
	}

	return ""
}

func (t *Transport) makeSubscriber(name string) (chan []byte, error) {
	region := RegionFromURL(name)
	svc, err := t.NewService(region)
	if err != nil {
		return nil, err
	}

	ch := make(chan []byte, 1024)

	params := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(name),
		MaxNumberOfMessages: aws.Int64(1),
		VisibilityTimeout:   aws.Int64(1),
		WaitTimeSeconds:     aws.Int64(1),
	}

	go func() {
		for {
			select {
			case <-t.stopSubChan:
				return
			default:
				resp, err := svc.ReceiveMessage(params)
				if err != nil {
					t.errChan <- &vice.Err{Name: name, Err: err}
					continue
				}

				if len(resp.Messages) > 0 {
					for _, m := range resp.Messages {
						if m.ReceiptHandle != nil {
							delParams := &sqs.DeleteMessageInput{
								QueueUrl:      aws.String(name),
								ReceiptHandle: aws.String(*m.ReceiptHandle),
							}
							_, err := svc.DeleteMessage(delParams)
							if err != nil {
								t.errChan <- &vice.Err{Name: name, Err: err}
								continue
							}
						}
						ch <- []byte(*m.Body)
					}
				}
			}
		}
	}()
	return ch, nil
}

// Send gets a channel on which messages with the
// specified name may be sent. The name is the queue's
// URL
func (t *Transport) Send(name string) chan<- []byte {
	t.sm.Lock()
	defer t.sm.Unlock()

	ch, ok := t.sendChans[name]
	if ok {
		return ch
	}

	ch, err := t.makePublisher(name)
	if err != nil {
		t.errChan <- &vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	t.sendChans[name] = ch
	return ch
}

func (t *Transport) makePublisher(name string) (chan []byte, error) {
	region := RegionFromURL(name)
	svc, err := t.NewService(region)
	if err != nil {
		return nil, err
	}

	ch := make(chan []byte, 1024)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		var accum []*sqs.SendMessageBatchRequestEntry

		defer func() {
			if t.batchSize == 0 && len(accum) == 0 {
				return
			}

			t.sendBatch(svc, name, accum)
			accum = make([]*sqs.SendMessageBatchRequestEntry, 0, t.batchSize)
		}()
		for {
			select {
			case <-t.stopPubChan:
				if len(ch) != 0 {
					continue
				}
				return

			case msg := <-ch:
				if t.batchSize == 0 {
					params := &sqs.SendMessageInput{
						MessageBody: aws.String(string(msg)),
						QueueUrl:    aws.String(name),
					}
					_, err := svc.SendMessage(params)
					if err != nil {
						t.errChan <- &vice.Err{Message: msg, Name: name, Err: err}
					}
					continue
				}

				id := fmt.Sprintf("%d", time.Now().UnixNano())
				accum = append(accum, &sqs.SendMessageBatchRequestEntry{
					MessageBody: aws.String(string(msg)),
					Id:          aws.String(id),
				})
				if len(accum) < t.batchSize {
					continue
				}

				t.sendBatch(svc, name, accum)
				accum = make([]*sqs.SendMessageBatchRequestEntry, 0, t.batchSize)
			case <-time.After(t.batchInterval):
				if len(accum) == 0 {
					continue
				}

				t.sendBatch(svc, name, accum)
				accum = make([]*sqs.SendMessageBatchRequestEntry, 0, t.batchSize)
			}
		}
	}()

	return ch, nil
}

func (t *Transport) sendBatch(svc sqsiface.SQSAPI, name string, entries []*sqs.SendMessageBatchRequestEntry) {
	batchParams := &sqs.SendMessageBatchInput{
		Entries:  entries,
		QueueUrl: aws.String(name),
	}

	resp, err := svc.SendMessageBatch(batchParams)
	if err != nil {
		t.errChan <- &vice.Err{Name: name, Err: err}
		return
	}

	for _, v := range resp.Failed {
		err := fmt.Errorf("%s", *v.Message)
		t.errChan <- &vice.Err{Name: name, Err: err}
	}
}

// ErrChan gets the channel on which errors are sent.
func (t *Transport) ErrChan() <-chan error {
	return t.errChan
}

// Stop stops the transport.
// The channel returned from Done() will be closed
// when the transport has stopped.
func (t *Transport) Stop() {
	close(t.stopSubChan)
	close(t.stopPubChan)
	t.wg.Wait()
	close(t.stopchan)
}

// Done gets a channel which is closed when the
// transport has successfully stopped.
func (t *Transport) Done() chan struct{} {
	return t.stopchan
}
