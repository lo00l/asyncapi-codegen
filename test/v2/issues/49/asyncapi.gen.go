// Package "issue49" provides primitives to interact with the AsyncAPI specification.
//
// Code generated by github.com/lerenn/asyncapi-codegen version (devel) DO NOT EDIT.
package issue49

import (
	"context"
	"fmt"

	"github.com/lerenn/asyncapi-codegen/pkg/extensions"
)

// AppSubscriber represents all handlers that are expecting messages for App
type AppSubscriber interface {
	// Issue49Chat subscribes to messages placed on the 'issue49.chat' channel
	Issue49Chat(ctx context.Context, msg Issue49ChatMessage)
}

// AppController is the structure that provides publishing capabilities to the
// developer and and connect the broker with the App
type AppController struct {
	controller
}

// NewAppController links the App to the broker
func NewAppController(bc extensions.BrokerController, options ...ControllerOption) (*AppController, error) {
	// Check if broker controller has been provided
	if bc == nil {
		return nil, extensions.ErrNilBrokerController
	}

	// Create default controller
	controller := controller{
		broker:        bc,
		subscriptions: make(map[string]extensions.BrokerChannelSubscription),
		logger:        extensions.DummyLogger{},
		middlewares:   make([]extensions.Middleware, 0),
	}

	// Apply options
	for _, option := range options {
		option(&controller)
	}

	return &AppController{controller: controller}, nil
}

func (c AppController) wrapMiddlewares(
	middlewares []extensions.Middleware,
	callback extensions.NextMiddleware,
) func(ctx context.Context, msg *extensions.BrokerMessage) error {
	var called bool

	// If there is no more middleware
	if len(middlewares) == 0 {
		return func(ctx context.Context, msg *extensions.BrokerMessage) error {
			// Call the callback if it exists and it has not been called already
			if callback != nil && !called {
				called = true
				return callback(ctx)
			}

			// Nil can be returned, as the callback has already been called
			return nil
		}
	}

	// Get the next function to call from next middlewares or callback
	next := c.wrapMiddlewares(middlewares[1:], callback)

	// Wrap middleware into a check function that will call execute the middleware
	// and call the next wrapped middleware if the returned function has not been
	// called already
	return func(ctx context.Context, msg *extensions.BrokerMessage) error {
		// Call the middleware and the following if it has not been done already
		if !called {
			// Create the next call with the context and the message
			nextWithArgs := func(ctx context.Context) error {
				return next(ctx, msg)
			}

			// Call the middleware and register it as already called
			called = true
			if err := middlewares[0](ctx, msg, nextWithArgs); err != nil {
				return err
			}

			// If next has already been called in middleware, it should not be executed again
			return nextWithArgs(ctx)
		}

		// Nil can be returned, as the next middleware has already been called
		return nil
	}
}

func (c AppController) executeMiddlewares(ctx context.Context, msg *extensions.BrokerMessage, callback extensions.NextMiddleware) error {
	// Wrap middleware to have 'next' function when calling them
	wrapped := c.wrapMiddlewares(c.middlewares, callback)

	// Execute wrapped middlewares
	return wrapped(ctx, msg)
}

func addAppContextValues(ctx context.Context, path string) context.Context {
	ctx = context.WithValue(ctx, extensions.ContextKeyIsVersion, "1.0.0")
	ctx = context.WithValue(ctx, extensions.ContextKeyIsProvider, "app")
	return context.WithValue(ctx, extensions.ContextKeyIsChannel, path)
}

// Close will clean up any existing resources on the controller
func (c *AppController) Close(ctx context.Context) {
	// Unsubscribing remaining channels
	c.UnsubscribeAll(ctx)

	c.logger.Info(ctx, "Closed app controller")
}

// SubscribeAll will subscribe to channels without parameters on which the app is expecting messages.
// For channels with parameters, they should be subscribed independently.
func (c *AppController) SubscribeAll(ctx context.Context, as AppSubscriber) error {
	if as == nil {
		return extensions.ErrNilAppSubscriber
	}

	if err := c.SubscribeIssue49Chat(ctx, as.Issue49Chat); err != nil {
		return err
	}

	return nil
}

// UnsubscribeAll will unsubscribe all remaining subscribed channels
func (c *AppController) UnsubscribeAll(ctx context.Context) {
	c.UnsubscribeIssue49Chat(ctx)
}

// SubscribeIssue49Chat will subscribe to new messages from 'issue49.chat' channel.
//
// Callback function 'fn' will be called each time a new message is received.
func (c *AppController) SubscribeIssue49Chat(ctx context.Context, fn func(ctx context.Context, msg Issue49ChatMessage)) error {
	// Get channel path
	path := "issue49.chat"

	// Set context
	ctx = addAppContextValues(ctx, path)
	ctx = context.WithValue(ctx, extensions.ContextKeyIsDirection, "reception")

	// Check if there is already a subscription
	_, exists := c.subscriptions[path]
	if exists {
		err := fmt.Errorf("%w: %q channel is already subscribed", extensions.ErrAlreadySubscribedChannel, path)
		c.logger.Error(ctx, err.Error())
		return err
	}

	// Subscribe to broker channel
	sub, err := c.broker.Subscribe(ctx, path)
	if err != nil {
		c.logger.Error(ctx, err.Error())
		return err
	}
	c.logger.Info(ctx, "Subscribed to channel")

	// Asynchronously listen to new messages and pass them to app subscriber
	go func() {
		for {
			// Wait for next message
			brokerMsg, open := <-sub.MessagesChannel()

			// If subscription is closed and there is no more message
			// (i.e. uninitialized message), then exit the function
			if !open && brokerMsg.IsUninitialized() {
				return
			}

			// Set broker message to context
			ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, brokerMsg.String())

			// Execute middlewares before handling the message
			if err := c.executeMiddlewares(ctx, &brokerMsg, func(ctx context.Context) error {
				// Process message
				msg, err := newIssue49ChatMessageFromBrokerMessage(brokerMsg)
				if err != nil {
					return err
				}

				// Execute the subscription function
				fn(ctx, msg)

				return nil
			}); err != nil {
				c.logger.Error(ctx, err.Error())
			}
		}
	}()

	// Add the cancel channel to the inside map
	c.subscriptions[path] = sub

	return nil
}

// UnsubscribeIssue49Chat will unsubscribe messages from 'issue49.chat' channel.
// A timeout can be set in context to avoid blocking operation, if needed.
func (c *AppController) UnsubscribeIssue49Chat(ctx context.Context) {
	// Get channel path
	path := "issue49.chat"

	// Check if there subscribers for this channel
	sub, exists := c.subscriptions[path]
	if !exists {
		return
	}

	// Set context
	ctx = addAppContextValues(ctx, path)

	// Stop the subscription
	sub.Cancel(ctx)

	// Remove if from the subscribers
	delete(c.subscriptions, path)

	c.logger.Info(ctx, "Unsubscribed from channel")
}

// PublishIssue49Chat will publish messages to 'issue49.chat' channel
func (c *AppController) PublishIssue49Chat(ctx context.Context, msg Issue49ChatMessage) error {
	// Get channel path
	path := "issue49.chat"

	// Set context
	ctx = addAppContextValues(ctx, path)
	ctx = context.WithValue(ctx, extensions.ContextKeyIsDirection, "publication")

	// Convert to BrokerMessage
	brokerMsg, err := msg.toBrokerMessage()
	if err != nil {
		return err
	}

	// Set broker message to context
	ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, brokerMsg.String())

	// Publish the message on event-broker through middlewares
	return c.executeMiddlewares(ctx, &brokerMsg, func(ctx context.Context) error {
		return c.broker.Publish(ctx, path, brokerMsg)
	})
}

// PublishIssue49Status will publish messages to 'issue49.status' channel
func (c *AppController) PublishIssue49Status(ctx context.Context, msg Issue49StatusMessage) error {
	// Get channel path
	path := "issue49.status"

	// Set context
	ctx = addAppContextValues(ctx, path)
	ctx = context.WithValue(ctx, extensions.ContextKeyIsDirection, "publication")

	// Convert to BrokerMessage
	brokerMsg, err := msg.toBrokerMessage()
	if err != nil {
		return err
	}

	// Set broker message to context
	ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, brokerMsg.String())

	// Publish the message on event-broker through middlewares
	return c.executeMiddlewares(ctx, &brokerMsg, func(ctx context.Context) error {
		return c.broker.Publish(ctx, path, brokerMsg)
	})
}

// UserSubscriber represents all handlers that are expecting messages for User
type UserSubscriber interface {
	// Issue49Chat subscribes to messages placed on the 'issue49.chat' channel
	Issue49Chat(ctx context.Context, msg Issue49ChatMessage)

	// Issue49Status subscribes to messages placed on the 'issue49.status' channel
	Issue49Status(ctx context.Context, msg Issue49StatusMessage)
}

// UserController is the structure that provides publishing capabilities to the
// developer and and connect the broker with the User
type UserController struct {
	controller
}

// NewUserController links the User to the broker
func NewUserController(bc extensions.BrokerController, options ...ControllerOption) (*UserController, error) {
	// Check if broker controller has been provided
	if bc == nil {
		return nil, extensions.ErrNilBrokerController
	}

	// Create default controller
	controller := controller{
		broker:        bc,
		subscriptions: make(map[string]extensions.BrokerChannelSubscription),
		logger:        extensions.DummyLogger{},
		middlewares:   make([]extensions.Middleware, 0),
	}

	// Apply options
	for _, option := range options {
		option(&controller)
	}

	return &UserController{controller: controller}, nil
}

func (c UserController) wrapMiddlewares(
	middlewares []extensions.Middleware,
	callback extensions.NextMiddleware,
) func(ctx context.Context, msg *extensions.BrokerMessage) error {
	var called bool

	// If there is no more middleware
	if len(middlewares) == 0 {
		return func(ctx context.Context, msg *extensions.BrokerMessage) error {
			// Call the callback if it exists and it has not been called already
			if callback != nil && !called {
				called = true
				return callback(ctx)
			}

			// Nil can be returned, as the callback has already been called
			return nil
		}
	}

	// Get the next function to call from next middlewares or callback
	next := c.wrapMiddlewares(middlewares[1:], callback)

	// Wrap middleware into a check function that will call execute the middleware
	// and call the next wrapped middleware if the returned function has not been
	// called already
	return func(ctx context.Context, msg *extensions.BrokerMessage) error {
		// Call the middleware and the following if it has not been done already
		if !called {
			// Create the next call with the context and the message
			nextWithArgs := func(ctx context.Context) error {
				return next(ctx, msg)
			}

			// Call the middleware and register it as already called
			called = true
			if err := middlewares[0](ctx, msg, nextWithArgs); err != nil {
				return err
			}

			// If next has already been called in middleware, it should not be executed again
			return nextWithArgs(ctx)
		}

		// Nil can be returned, as the next middleware has already been called
		return nil
	}
}

func (c UserController) executeMiddlewares(ctx context.Context, msg *extensions.BrokerMessage, callback extensions.NextMiddleware) error {
	// Wrap middleware to have 'next' function when calling them
	wrapped := c.wrapMiddlewares(c.middlewares, callback)

	// Execute wrapped middlewares
	return wrapped(ctx, msg)
}

func addUserContextValues(ctx context.Context, path string) context.Context {
	ctx = context.WithValue(ctx, extensions.ContextKeyIsVersion, "1.0.0")
	ctx = context.WithValue(ctx, extensions.ContextKeyIsProvider, "user")
	return context.WithValue(ctx, extensions.ContextKeyIsChannel, path)
}

// Close will clean up any existing resources on the controller
func (c *UserController) Close(ctx context.Context) {
	// Unsubscribing remaining channels
	c.UnsubscribeAll(ctx)

	c.logger.Info(ctx, "Closed user controller")
}

// SubscribeAll will subscribe to channels without parameters on which the app is expecting messages.
// For channels with parameters, they should be subscribed independently.
func (c *UserController) SubscribeAll(ctx context.Context, as UserSubscriber) error {
	if as == nil {
		return extensions.ErrNilUserSubscriber
	}

	if err := c.SubscribeIssue49Chat(ctx, as.Issue49Chat); err != nil {
		return err
	}
	if err := c.SubscribeIssue49Status(ctx, as.Issue49Status); err != nil {
		return err
	}

	return nil
}

// UnsubscribeAll will unsubscribe all remaining subscribed channels
func (c *UserController) UnsubscribeAll(ctx context.Context) {
	c.UnsubscribeIssue49Chat(ctx)
	c.UnsubscribeIssue49Status(ctx)
}

// SubscribeIssue49Chat will subscribe to new messages from 'issue49.chat' channel.
//
// Callback function 'fn' will be called each time a new message is received.
func (c *UserController) SubscribeIssue49Chat(ctx context.Context, fn func(ctx context.Context, msg Issue49ChatMessage)) error {
	// Get channel path
	path := "issue49.chat"

	// Set context
	ctx = addUserContextValues(ctx, path)
	ctx = context.WithValue(ctx, extensions.ContextKeyIsDirection, "reception")

	// Check if there is already a subscription
	_, exists := c.subscriptions[path]
	if exists {
		err := fmt.Errorf("%w: %q channel is already subscribed", extensions.ErrAlreadySubscribedChannel, path)
		c.logger.Error(ctx, err.Error())
		return err
	}

	// Subscribe to broker channel
	sub, err := c.broker.Subscribe(ctx, path)
	if err != nil {
		c.logger.Error(ctx, err.Error())
		return err
	}
	c.logger.Info(ctx, "Subscribed to channel")

	// Asynchronously listen to new messages and pass them to app subscriber
	go func() {
		for {
			// Wait for next message
			brokerMsg, open := <-sub.MessagesChannel()

			// If subscription is closed and there is no more message
			// (i.e. uninitialized message), then exit the function
			if !open && brokerMsg.IsUninitialized() {
				return
			}

			// Set broker message to context
			ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, brokerMsg.String())

			// Execute middlewares before handling the message
			if err := c.executeMiddlewares(ctx, &brokerMsg, func(ctx context.Context) error {
				// Process message
				msg, err := newIssue49ChatMessageFromBrokerMessage(brokerMsg)
				if err != nil {
					return err
				}

				// Execute the subscription function
				fn(ctx, msg)

				return nil
			}); err != nil {
				c.logger.Error(ctx, err.Error())
			}
		}
	}()

	// Add the cancel channel to the inside map
	c.subscriptions[path] = sub

	return nil
}

// UnsubscribeIssue49Chat will unsubscribe messages from 'issue49.chat' channel.
// A timeout can be set in context to avoid blocking operation, if needed.
func (c *UserController) UnsubscribeIssue49Chat(ctx context.Context) {
	// Get channel path
	path := "issue49.chat"

	// Check if there subscribers for this channel
	sub, exists := c.subscriptions[path]
	if !exists {
		return
	}

	// Set context
	ctx = addUserContextValues(ctx, path)

	// Stop the subscription
	sub.Cancel(ctx)

	// Remove if from the subscribers
	delete(c.subscriptions, path)

	c.logger.Info(ctx, "Unsubscribed from channel")
} // SubscribeIssue49Status will subscribe to new messages from 'issue49.status' channel.
// Callback function 'fn' will be called each time a new message is received.
func (c *UserController) SubscribeIssue49Status(ctx context.Context, fn func(ctx context.Context, msg Issue49StatusMessage)) error {
	// Get channel path
	path := "issue49.status"

	// Set context
	ctx = addUserContextValues(ctx, path)
	ctx = context.WithValue(ctx, extensions.ContextKeyIsDirection, "reception")

	// Check if there is already a subscription
	_, exists := c.subscriptions[path]
	if exists {
		err := fmt.Errorf("%w: %q channel is already subscribed", extensions.ErrAlreadySubscribedChannel, path)
		c.logger.Error(ctx, err.Error())
		return err
	}

	// Subscribe to broker channel
	sub, err := c.broker.Subscribe(ctx, path)
	if err != nil {
		c.logger.Error(ctx, err.Error())
		return err
	}
	c.logger.Info(ctx, "Subscribed to channel")

	// Asynchronously listen to new messages and pass them to app subscriber
	go func() {
		for {
			// Wait for next message
			brokerMsg, open := <-sub.MessagesChannel()

			// If subscription is closed and there is no more message
			// (i.e. uninitialized message), then exit the function
			if !open && brokerMsg.IsUninitialized() {
				return
			}

			// Set broker message to context
			ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, brokerMsg.String())

			// Execute middlewares before handling the message
			if err := c.executeMiddlewares(ctx, &brokerMsg, func(ctx context.Context) error {
				// Process message
				msg, err := newIssue49StatusMessageFromBrokerMessage(brokerMsg)
				if err != nil {
					return err
				}

				// Execute the subscription function
				fn(ctx, msg)

				return nil
			}); err != nil {
				c.logger.Error(ctx, err.Error())
			}
		}
	}()

	// Add the cancel channel to the inside map
	c.subscriptions[path] = sub

	return nil
}

// UnsubscribeIssue49Status will unsubscribe messages from 'issue49.status' channel.
// A timeout can be set in context to avoid blocking operation, if needed.
func (c *UserController) UnsubscribeIssue49Status(ctx context.Context) {
	// Get channel path
	path := "issue49.status"

	// Check if there subscribers for this channel
	sub, exists := c.subscriptions[path]
	if !exists {
		return
	}

	// Set context
	ctx = addUserContextValues(ctx, path)

	// Stop the subscription
	sub.Cancel(ctx)

	// Remove if from the subscribers
	delete(c.subscriptions, path)

	c.logger.Info(ctx, "Unsubscribed from channel")
}

// PublishIssue49Chat will publish messages to 'issue49.chat' channel
func (c *UserController) PublishIssue49Chat(ctx context.Context, msg Issue49ChatMessage) error {
	// Get channel path
	path := "issue49.chat"

	// Set context
	ctx = addUserContextValues(ctx, path)
	ctx = context.WithValue(ctx, extensions.ContextKeyIsDirection, "publication")

	// Convert to BrokerMessage
	brokerMsg, err := msg.toBrokerMessage()
	if err != nil {
		return err
	}

	// Set broker message to context
	ctx = context.WithValue(ctx, extensions.ContextKeyIsBrokerMessage, brokerMsg.String())

	// Publish the message on event-broker through middlewares
	return c.executeMiddlewares(ctx, &brokerMsg, func(ctx context.Context) error {
		return c.broker.Publish(ctx, path, brokerMsg)
	})
}

// AsyncAPIVersion is the version of the used AsyncAPI document
const AsyncAPIVersion = "1.0.0"

// controller is the controller that will be used to communicate with the broker
// It will be used internally by AppController and UserController
type controller struct {
	// broker is the broker controller that will be used to communicate
	broker extensions.BrokerController
	// subscriptions is a map of all subscriptions
	subscriptions map[string]extensions.BrokerChannelSubscription
	// logger is the logger that will be used² to log operations on controller
	logger extensions.Logger
	// middlewares are the middlewares that will be executed when sending or
	// receiving messages
	middlewares []extensions.Middleware
}

// ControllerOption is the type of the options that can be passed
// when creating a new Controller
type ControllerOption func(controller *controller)

// WithLogger attaches a logger to the controller
func WithLogger(logger extensions.Logger) ControllerOption {
	return func(controller *controller) {
		controller.logger = logger
	}
}

// WithMiddlewares attaches middlewares that will be executed when sending or receiving messages
func WithMiddlewares(middlewares ...extensions.Middleware) ControllerOption {
	return func(controller *controller) {
		controller.middlewares = middlewares
	}
}

type MessageWithCorrelationID interface {
	CorrelationID() string
	SetCorrelationID(id string)
}

type Error struct {
	Channel string
	Err     error
}

func (e *Error) Error() string {
	return fmt.Sprintf("channel %q: err %v", e.Channel, e.Err)
}

// Issue49ChatMessage is the message expected for 'Issue49Chat' channel
type Issue49ChatMessage struct {
	// Payload will be inserted in the message payload
	Payload string
}

func NewIssue49ChatMessage() Issue49ChatMessage {
	var msg Issue49ChatMessage

	return msg
}

// newIssue49ChatMessageFromBrokerMessage will fill a new Issue49ChatMessage with data from generic broker message
func newIssue49ChatMessageFromBrokerMessage(bMsg extensions.BrokerMessage) (Issue49ChatMessage, error) {
	var msg Issue49ChatMessage

	// Convert to string
	payload := string(bMsg.Payload)
	msg.Payload = payload // No need for type conversion to reference

	// TODO: run checks on msg type

	return msg, nil
}

// toBrokerMessage will generate a generic broker message from Issue49ChatMessage data
func (msg Issue49ChatMessage) toBrokerMessage() (extensions.BrokerMessage, error) {
	// TODO: implement checks on message

	// Convert to []byte
	payload := []byte(msg.Payload)

	// There is no headers here
	headers := make(map[string][]byte, 0)

	return extensions.BrokerMessage{
		Headers: headers,
		Payload: payload,
	}, nil
}

// Message is the message expected for ” channel
type Message struct {
	// Payload will be inserted in the message payload
	Payload string
}

func NewMessage() Message {
	var msg Message

	return msg
}

// newMessageFromBrokerMessage will fill a new Message with data from generic broker message
func newMessageFromBrokerMessage(bMsg extensions.BrokerMessage) (Message, error) {
	var msg Message

	// Convert to string
	payload := string(bMsg.Payload)
	msg.Payload = payload // No need for type conversion to reference

	// TODO: run checks on msg type

	return msg, nil
}

// toBrokerMessage will generate a generic broker message from Message data
func (msg Message) toBrokerMessage() (extensions.BrokerMessage, error) {
	// TODO: implement checks on message

	// Convert to []byte
	payload := []byte(msg.Payload)

	// There is no headers here
	headers := make(map[string][]byte, 0)

	return extensions.BrokerMessage{
		Headers: headers,
		Payload: payload,
	}, nil
}

// Issue49StatusMessage is the message expected for 'Issue49Status' channel
type Issue49StatusMessage struct {
	// Payload will be inserted in the message payload
	Payload string
}

func NewIssue49StatusMessage() Issue49StatusMessage {
	var msg Issue49StatusMessage

	return msg
}

// newIssue49StatusMessageFromBrokerMessage will fill a new Issue49StatusMessage with data from generic broker message
func newIssue49StatusMessageFromBrokerMessage(bMsg extensions.BrokerMessage) (Issue49StatusMessage, error) {
	var msg Issue49StatusMessage

	// Convert to string
	payload := string(bMsg.Payload)
	msg.Payload = payload // No need for type conversion to reference

	// TODO: run checks on msg type

	return msg, nil
}

// toBrokerMessage will generate a generic broker message from Issue49StatusMessage data
func (msg Issue49StatusMessage) toBrokerMessage() (extensions.BrokerMessage, error) {
	// TODO: implement checks on message

	// Convert to []byte
	payload := []byte(msg.Payload)

	// There is no headers here
	headers := make(map[string][]byte, 0)

	return extensions.BrokerMessage{
		Headers: headers,
		Payload: payload,
	}, nil
}

const (
	// Issue49ChatPath is the constant representing the 'Issue49.chat' channel path.
	Issue49ChatPath = "issue49.chat"
	// Issue49StatusPath is the constant representing the 'Issue49.status' channel path.
	Issue49StatusPath = "issue49.status"
)

// ChannelsPaths is an array of all channels paths
var ChannelsPaths = []string{
	Issue49ChatPath,
	Issue49StatusPath,
}