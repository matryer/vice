package sqs

import (
	"fmt"
	"sync"
	"time"

	"github.com/matryer/vice"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

// MultiTransport is a vice.Transport for Amazon's SQS
type MultiTransport struct {
	writers       int
	batchSize     int
	batchInterval time.Duration

	wg sync.WaitGroup

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

var _ vice.Transport = (*MultiTransport)(nil)

// NewMulti returns a new transport with multiple sqs writers
// Credentials are automatically sourced using the AWS SDK credential chain,
// for more info see the AWS SDK docs:
// https://godoc.org/github.com/aws/aws-sdk-go#hdr-Configuring_Credentials
func NewMulti(writers, batchSize int, batchInterval time.Duration) *MultiTransport {
	const defaultWriter = 2
	if writers == 0 {
		writers = defaultWriter
	}

	const maxBatchSize = 10
	if batchSize > 10 {
		batchSize = maxBatchSize
	}

	const defaultInterval = 200 * time.Millisecond
	if batchInterval == 0 {
		batchInterval = defaultInterval
	}

	return &MultiTransport{
		writers:       writers,
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
func (t *MultiTransport) Receive(name string) <-chan []byte {
	t.rm.Lock()
	defer t.rm.Unlock()

	ch, ok := t.receiveChans[name]
	if ok {
		return ch
	}

	ch, err := t.makeSubscriber(name)
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	t.receiveChans[name] = ch
	return ch
}

func (t *MultiTransport) makeSubscriber(name string) (chan []byte, error) {
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
					t.errChan <- vice.Err{Name: name, Err: err}
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
								t.errChan <- vice.Err{Name: name, Err: err}
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
func (t *MultiTransport) Send(name string) chan<- []byte {
	t.sm.Lock()
	defer t.sm.Unlock()

	ch, ok := t.sendChans[name]
	if ok {
		return ch
	}

	ch, err := t.makePublishers(name)
	if err != nil {
		t.errChan <- vice.Err{Name: name, Err: err}
		return make(chan []byte)
	}

	t.sendChans[name] = ch
	return ch
}

// makePublishers creates t.writers of writers to make outgoing requests to the
// SQS queue. This scales horizontally as doing single publisher is very limited.
// This method will create the publishers and use the first available publisher
// that is not making a send to SQS. If all are active then it will fallback block.
// The sends to SQS are now done concurrently as well to remove the sends from the
// critical path. This is ideal as we are able to increase throughput and still
// maintain the error channel to share errors with downstream components.
func (t *MultiTransport) makePublishers(name string) (chan []byte, error) {
	region := RegionFromURL(name)

	queue := make(chan writerMsg)
	err := newWriterGroup(t.writers, t.stopchan, t.errChan, queue, t.NewService, region)
	if err != nil {
		return nil, err
	}

	ch := make(chan []byte, 1024)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		var accum sendMsgBatch

		defer func() {
			if len(accum) == 0 {
				return
			}
			queue <- writerMsg{batch: accum, name: name}
			time.Sleep(50 * time.Millisecond)
		}()

		for {
			select {
			case <-t.stopPubChan:
				if len(ch) != 0 {
					continue
				}
				return
			case msg := <-ch:
				id := fmt.Sprintf("%d", time.Now().UnixNano())
				accum = append(accum, sqs.SendMessageBatchRequestEntry{
					MessageBody: aws.String(string(msg)),
					Id:          aws.String(id),
				})

				if t.batchSize == 0 {
					tmp := make(sendMsgBatch, len(accum))
					copy(tmp, accum)
					queue <- writerMsg{batch: tmp, name: name}
					accum = make(sendMsgBatch, 0, t.batchSize)
					continue
				}

				if len(accum) < t.batchSize {
					continue
				}

				tmp := make(sendMsgBatch, len(accum))
				copy(tmp, accum)
				queue <- writerMsg{batch: tmp, name: name}
				accum = make(sendMsgBatch, 0, t.batchSize)
			case <-time.After(t.batchInterval):
				if t.batchSize == 0 || len(accum) == 0 {
					continue
				}

				tmp := make(sendMsgBatch, len(accum))
				copy(tmp, accum)
				queue <- writerMsg{batch: tmp, name: name}
				accum = make(sendMsgBatch, 0, t.batchSize)
			}
		}
	}()

	return ch, nil
}

// ErrChan gets the channel on which errors are sent.
func (t *MultiTransport) ErrChan() <-chan error {
	return t.errChan
}

// Stop stops the transport.
// The channel returned from Done() will be closed
// when the transport has stopped.
func (t *MultiTransport) Stop() {
	close(t.stopSubChan)
	close(t.stopPubChan)
	t.wg.Wait()
	close(t.stopchan)
}

// Done gets a channel which is closed when the
// transport has successfully stopped.
func (t *MultiTransport) Done() chan struct{} {
	return t.stopchan
}

type sendMsgBatch []sqs.SendMessageBatchRequestEntry

type writerMsg struct {
	batch sendMsgBatch
	name  string
}

type newServiceFn func(region string) (sqsiface.SQSAPI, error)

func newWriterGroup(n int, done <-chan struct{}, errChan chan<- error, queue <-chan writerMsg, newSVCFn newServiceFn, region string) error {
	var svcs []sqsiface.SQSAPI
	for i := 0; i < n; i++ {
		svc, err := newSVCFn(region)
		if err != nil {
			return err
		}
		svcs = append(svcs, svc)
	}

	workerQueue := make(chan chan writerMsg, n)
	for i := range svcs {
		w := newWriter(svcs[i], done, errChan, workerQueue)
		w.Start()
	}

	go func() {
		for work := range queue {
			select {
			case <-done:
				return
			default:
				worker := <-workerQueue
				worker <- work
			}
		}
	}()

	return nil
}

type svcWriter struct {
	svc         sqsiface.SQSAPI
	work        chan writerMsg
	workerQueue chan chan writerMsg
	done        <-chan struct{}
	errChan     chan<- error
}

func newWriter(svc sqsiface.SQSAPI, done <-chan struct{}, errChan chan<- error, workerQueue chan chan writerMsg) *svcWriter {
	return &svcWriter{
		svc:         svc,
		work:        make(chan writerMsg),
		workerQueue: workerQueue,
		done:        done,
		errChan:     errChan,
	}
}

func (s *svcWriter) Start() {
	go func() {
		for {
			s.workerQueue <- s.work

			select {
			case <-s.done:
				return
			case w := <-s.work:
				if len(w.batch) == 1 {
					s.sendMessage(w.name, w.batch[0])
					continue
				}
				s.sendBatch(w.name, w.batch)
			}
		}
	}()
}

func (s *svcWriter) sendMessage(name string, msg sqs.SendMessageBatchRequestEntry) {
	_, err := s.svc.SendMessage(&sqs.SendMessageInput{
		MessageBody: msg.MessageBody,
		QueueUrl:    aws.String(name),
	})
	if err != nil {
		s.errChan <- vice.Err{Message: []byte(*msg.MessageBody), Name: name, Err: err}
	}
}

func (s *svcWriter) sendBatch(name string, batch sendMsgBatch) {
	if len(batch) == 0 {
		return
	}

	entryPtrs := make([]*sqs.SendMessageBatchRequestEntry, len(batch))
	for i := range batch {
		entryPtrs[i] = &batch[i]
	}

	batchParams := &sqs.SendMessageBatchInput{
		Entries:  entryPtrs,
		QueueUrl: aws.String(name),
	}

	resp, err := s.svc.SendMessageBatch(batchParams)
	if err != nil {
		s.errChan <- vice.Err{Name: name, Err: err}
		return
	}

	for _, v := range resp.Failed {
		s.errChan <- vice.Err{Name: name, Err: fmt.Errorf("%s", *v.Message)}
	}
}
