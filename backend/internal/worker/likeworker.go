package worker

import (
	"context"
	"encoding/json"
	"errors"
	"feedsystem_video_go/internal/middleware/rabbitmq"
	"feedsystem_video_go/internal/video"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// consumeConcurrency 单个队列的并发消费 goroutine 数
const consumeConcurrency = 8

type LikeWorker struct {
	ch    *amqp.Channel
	likes *video.LikeRepository
	queue string
}

func NewLikeWorker(ch *amqp.Channel, likes *video.LikeRepository, queue string) *LikeWorker {
	return &LikeWorker{ch: ch, likes: likes, queue: queue}
}

func (w *LikeWorker) Run(ctx context.Context) error {
	if w == nil || w.ch == nil || w.likes == nil {
		return errors.New("like worker is not initialized")
	}
	if w.queue == "" {
		return errors.New("queue is required")
	}

	deliveries, err := w.ch.Consume(
		w.queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for i := 0; i < consumeConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case d, ok := <-deliveries:
					if !ok {
						return
					}
					w.handleDelivery(ctx, d)
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

func (w *LikeWorker) handleDelivery(ctx context.Context, d amqp.Delivery) {
	if err := w.process(ctx, d.Body); err != nil {
		log.Printf("like worker: failed to process message: %v", err)
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (w *LikeWorker) process(ctx context.Context, body []byte) error {
	var evt rabbitmq.LikeEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		// 解析事件失败，直接丢弃
		return nil
	}
	if evt.UserID == 0 || evt.VideoID == 0 {
		return nil
	}

	switch evt.Action {
	case "like":
		return w.applyLike(ctx, evt.UserID, evt.VideoID)
	case "unlike":
		return w.applyUnlike(ctx, evt.UserID, evt.VideoID)
	default:
		return nil
	}
}

// api层已经做过isLike校验, 所以这里直接加就好
func (w *LikeWorker) applyLike(ctx context.Context, userID, videoID uint) error {
	// like与popularity合并更新,一次提交
	created, err := w.likes.LikeAndBump(ctx, &video.Like{
		VideoID:   videoID,
		AccountID: userID,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	return nil
}

// api层已经做过存在性检查
func (w *LikeWorker) applyUnlike(ctx context.Context, userID, videoID uint) error {
	deleted, err := w.likes.UnlikeAndBump(ctx, videoID, userID)
	if err != nil {
		return err
	}
	_ = deleted
	return nil
}
