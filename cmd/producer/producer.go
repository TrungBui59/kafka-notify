package producer

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/TrungBui59/kafka-notify/pkg/models"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

const (
	ProducerPort    = "8080"
	KafkaServerAddr = "localhost:9092"
	KafkaTopic      = "notifications"
)

// custom error for user not found
var ErrUserNotFoundInProducer = errors.New("user not found")

func FindUserById(id int, users []models.User) (models.User, error) {
	for _, user := range users {
		if user.ID == id {
			return user, nil
		}
	}
	return models.User{}, ErrUserNotFoundInProducer
}

func getIDFromRequest(formValue string, ctx *gin.Context) (int, error) {
	id, err := strconv.Atoi(ctx.PostForm(formValue))
	if err != nil {
		return 0, fmt.Errorf(
			"failed to parse ID from form value %s: %w", formValue, err)
	}
	return id, nil
}

// kafka related issue
func sendKafkaMessage(
	producer sarama.SyncProducer,
	users []models.User,
	ctx *gin.Context,
	fromId, toId int,
) error {
	message := ctx.PostForm("message")

	// find the sender
	fromUser, err := FindUserById(fromId, users)
	if err != nil {
		return err
	}

	// find the receiver
	toUser, err := FindUserById(toId, users)
	if err != nil {
		return err
	}

	notification := models.Notification{
		From:    fromUser,
		To:      toUser,
		Message: message,
	}

	// json the notification
	notificationJson, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: KafkaTopic,
		Key:   sarama.StringEncoder(strconv.Itoa(toUser.ID)),
		Value: sarama.StringEncoder(notificationJson),
	}

	_, _, err = producer.SendMessage(msg)
	return err
}

func sendMessageHandler(producer sarama.SyncProducer,
	users []models.User) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		fromID, err := getIDFromRequest("fromID", ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}

		toID, err := getIDFromRequest("toID", ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}

		err = sendKafkaMessage(producer, users, ctx, fromID, toID)
		if errors.Is(err, ErrUserNotFoundInProducer) {
			ctx.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
			return
		}
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"message": "Notification sent successfully!",
		})
	}
}

// setup the producer
func setupProducer() (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer([]string{KafkaServerAddr},
		config)
	if err != nil {
		return nil, fmt.Errorf("failed to setup producer: %w", err)
	}
	return producer, nil
}
