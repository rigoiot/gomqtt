package client

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/256dpi/gomqtt/client/future"
	"github.com/256dpi/gomqtt/packet"
	"github.com/rigoiot/gomqtt/session"
	"github.com/256dpi/gomqtt/transport"
	"github.com/256dpi/gomqtt/transport/flow"
	"github.com/stretchr/testify/assert"
)

func TestClientConnectWrongURL(t *testing.T) {
	c := New()
	c.Callback = errorCallback(t)

	// wrong url
	connectFuture, err := c.Connect(NewConfig("foo"))
	assert.Error(t, err)
	assert.Nil(t, connectFuture)
}

func TestClientConnectWrongKeepAlive(t *testing.T) {
	c := New()
	c.Callback = errorCallback(t)

	// wrong keep alive
	connectFuture, err := c.Connect(&Config{
		BrokerURL:    "mqtt://localhost:1234",
		KeepAlive:    "foo",
		CleanSession: true,
	})
	assert.Error(t, err)
	assert.Nil(t, connectFuture)
}

func TestClientConnectErrorWrongPort(t *testing.T) {
	c := New()
	c.Callback = errorCallback(t)

	// wrong port
	connectFuture, err := c.Connect(NewConfig("mqtt://localhost:1234"))
	assert.Error(t, err)
	assert.Nil(t, connectFuture)
}

func TestClientConnectErrorMissingClientID(t *testing.T) {
	c := New()
	c.Callback = errorCallback(t)

	// missing clientID when clean=false
	connectFuture, err := c.Connect(NewConfig("mqtt://localhost:1234"))
	assert.Error(t, err)
	assert.Nil(t, connectFuture)
}

func TestClientConnect(t *testing.T) {
	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)
}

func TestClientConnectCustomDialer(t *testing.T) {
	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	config := NewConfig("tcp://localhost:" + port)
	config.Dialer = transport.NewDialer()

	connectFuture, err := c.Connect(config)
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)
}

func TestClientConnectAfterConnect(t *testing.T) {
	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	connectFuture, err = c.Connect(NewConfig("tcp://localhost:" + port))
	assert.Equal(t, ErrClientAlreadyConnecting, err)
	assert.Nil(t, connectFuture)

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)
}

func TestClientConnectWithCredentials(t *testing.T) {
	connect := connectPacket()
	connect.Username = "test"
	connect.Password = "test"

	broker := flow.New().
		Receive(connect).
		Send(connackPacket()).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	connectFuture, err := c.Connect(NewConfig(fmt.Sprintf("tcp://test:test@localhost:%s/", port)))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)
}

func TestClientNotConnected(t *testing.T) {
	c := New()
	c.Callback = errorCallback(t)

	future1, err := c.Publish("test", []byte("test"), 0, false)
	assert.Nil(t, future1)
	assert.Equal(t, ErrClientNotConnected, err)

	future2, err := c.Subscribe("test", 0)
	assert.Nil(t, future2)
	assert.Equal(t, ErrClientNotConnected, err)

	future3, err := c.Unsubscribe("test")
	assert.Nil(t, future3)
	assert.Equal(t, ErrClientNotConnected, err)

	err = c.Disconnect()
	assert.Equal(t, ErrClientNotConnected, err)

	err = c.Close()
	assert.Equal(t, ErrClientNotConnected, err)
}

func TestClientConnectionDenied(t *testing.T) {
	connack := connackPacket()
	connack.ReturnCode = packet.ErrNotAuthorized

	broker := flow.New().
		Receive(connectPacket()).
		Send(connack).
		Close()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.Nil(t, msg)
		assert.Equal(t, ErrClientConnectionDenied, err)
		close(wait)
		return nil
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.Error(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ErrNotAuthorized, connectFuture.ReturnCode())

	safeReceive(done)
	safeReceive(wait)
}

func TestClientExpectedConnack(t *testing.T) {
	broker := flow.New().
		Receive(connectPacket()).
		Send(packet.NewPingrespPacket()).
		End()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.Nil(t, msg)
		assert.Equal(t, ErrClientExpectedConnack, err)
		close(wait)
		return nil
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.Equal(t, future.ErrCanceled, connectFuture.Wait(1*time.Second))

	safeReceive(done)
	safeReceive(wait)
}

func TestClientKeepAlive(t *testing.T) {
	connect := connectPacket()
	connect.KeepAlive = 0

	pingreq := packet.NewPingreqPacket()
	pingresp := packet.NewPingrespPacket()

	broker := flow.New().
		Receive(connect).
		Send(connackPacket()).
		Receive(pingreq).
		Send(pingresp).
		Receive(pingreq).
		Send(pingresp).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	var reqCounter int32
	var respCounter int32

	c.Logger = func(message string) {
		if strings.Contains(message, "Pingreq") {
			atomic.AddInt32(&reqCounter, 1)
		} else if strings.Contains(message, "Pingresp") {
			atomic.AddInt32(&respCounter, 1)
		}
	}

	config := NewConfig("tcp://localhost:" + port)
	config.KeepAlive = "100ms"

	connectFuture, err := c.Connect(config)
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	<-time.After(250 * time.Millisecond)

	err = c.Disconnect()
	assert.NoError(t, err)

	assert.Equal(t, int32(2), atomic.LoadInt32(&reqCounter))
	assert.Equal(t, int32(2), atomic.LoadInt32(&respCounter))

	safeReceive(done)
}

func TestClientKeepAliveTimeout(t *testing.T) {
	connect := connectPacket()
	connect.KeepAlive = 0

	pingreq := packet.NewPingreqPacket()

	broker := flow.New().
		Receive(connect).
		Send(connackPacket()).
		Receive(pingreq).
		End()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.Nil(t, msg)
		assert.Equal(t, ErrClientMissingPong, err)
		close(wait)
		return nil
	}

	config := NewConfig("tcp://localhost:" + port)
	config.KeepAlive = "5ms"

	connectFuture, err := c.Connect(config)
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	safeReceive(wait)
	safeReceive(done)
}

func TestClientPublishSubscribeQOS0(t *testing.T) {
	subscribe := packet.NewSubscribePacket()
	subscribe.Subscriptions = []packet.Subscription{{Topic: "test"}}
	subscribe.ID = 1

	suback := packet.NewSubackPacket()
	suback.ReturnCodes = []uint8{0}
	suback.ID = 1

	publish := packet.NewPublishPacket()
	publish.Message.Topic = "test"
	publish.Message.Payload = []byte("test")

	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(subscribe).
		Send(suback).
		Receive(publish).
		Send(publish).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.NoError(t, err)
		assert.Equal(t, "test", msg.Topic)
		assert.Equal(t, []byte("test"), msg.Payload)
		assert.Equal(t, uint8(0), msg.QOS)
		assert.False(t, msg.Retain)
		close(wait)
		return nil
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	subscribeFuture, err := c.Subscribe("test", 0)
	assert.NoError(t, err)
	assert.NoError(t, subscribeFuture.Wait(1*time.Second))
	assert.Equal(t, []uint8{0}, subscribeFuture.ReturnCodes())

	publishFuture, err := c.Publish("test", []byte("test"), 0, false)
	assert.NoError(t, err)
	assert.NoError(t, publishFuture.Wait(1*time.Second))

	safeReceive(wait)

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)

	in, err := c.Session.AllPackets(session.Incoming)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(in))

	out, err := c.Session.AllPackets(session.Outgoing)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(out))
}

func TestClientPublishSubscribeQOS1(t *testing.T) {
	subscribe := packet.NewSubscribePacket()
	subscribe.Subscriptions = []packet.Subscription{{Topic: "test", QOS: 1}}
	subscribe.ID = 1

	suback := packet.NewSubackPacket()
	suback.ReturnCodes = []uint8{1}
	suback.ID = 1

	publish := packet.NewPublishPacket()
	publish.Message.Topic = "test"
	publish.Message.Payload = []byte("test")
	publish.Message.QOS = 1
	publish.ID = 2

	puback := packet.NewPubackPacket()
	puback.ID = 2

	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(subscribe).
		Send(suback).
		Receive(publish).
		Send(puback).
		Send(publish).
		Receive(puback).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.NoError(t, err)
		assert.Equal(t, "test", msg.Topic)
		assert.Equal(t, []byte("test"), msg.Payload)
		assert.Equal(t, uint8(1), msg.QOS)
		assert.False(t, msg.Retain)
		close(wait)
		return nil
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	subscribeFuture, err := c.Subscribe("test", 1)
	assert.NoError(t, err)
	assert.NoError(t, subscribeFuture.Wait(1*time.Second))
	assert.Equal(t, []uint8{1}, subscribeFuture.ReturnCodes())

	publishFuture, err := c.Publish("test", []byte("test"), 1, false)
	assert.NoError(t, err)
	assert.NoError(t, publishFuture.Wait(1*time.Second))

	safeReceive(wait)

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)

	in, err := c.Session.AllPackets(session.Incoming)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(in))

	out, err := c.Session.AllPackets(session.Outgoing)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(out))
}

func TestClientPublishSubscribeQOS2(t *testing.T) {
	subscribe := packet.NewSubscribePacket()
	subscribe.Subscriptions = []packet.Subscription{{Topic: "test", QOS: 2}}
	subscribe.ID = 1

	suback := packet.NewSubackPacket()
	suback.ReturnCodes = []uint8{2}
	suback.ID = 1

	publish := packet.NewPublishPacket()
	publish.Message.Topic = "test"
	publish.Message.Payload = []byte("test")
	publish.Message.QOS = 2
	publish.ID = 2

	pubrec := packet.NewPubrecPacket()
	pubrec.ID = 2

	pubrel := packet.NewPubrelPacket()
	pubrel.ID = 2

	pubcomp := packet.NewPubcompPacket()
	pubcomp.ID = 2

	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(subscribe).
		Send(suback).
		Receive(publish).
		Send(pubrec).
		Receive(pubrel).
		Send(pubcomp).
		Send(publish).
		Receive(pubrec).
		Send(pubrel).
		Receive(pubcomp).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.NoError(t, err)
		assert.Equal(t, "test", msg.Topic)
		assert.Equal(t, []byte("test"), msg.Payload)
		assert.Equal(t, uint8(2), msg.QOS)
		assert.False(t, msg.Retain)
		close(wait)
		return nil
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	subscribeFuture, err := c.Subscribe("test", 2)
	assert.NoError(t, err)
	assert.NoError(t, subscribeFuture.Wait(1*time.Second))
	assert.Equal(t, []uint8{2}, subscribeFuture.ReturnCodes())

	publishFuture, err := c.Publish("test", []byte("test"), 2, false)
	assert.NoError(t, err)
	assert.NoError(t, publishFuture.Wait(1*time.Second))

	safeReceive(wait)

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)

	in, err := c.Session.AllPackets(session.Incoming)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(in))

	out, err := c.Session.AllPackets(session.Outgoing)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(out))
}

func TestClientUnsubscribe(t *testing.T) {
	unsubscribe := packet.NewUnsubscribePacket()
	unsubscribe.Topics = []string{"test"}
	unsubscribe.ID = 1

	unsuback := packet.NewUnsubackPacket()
	unsuback.ID = 1

	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(unsubscribe).
		Send(unsuback).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	unsubscribeFuture, err := c.Unsubscribe("test")
	assert.NoError(t, err)
	assert.NoError(t, unsubscribeFuture.Wait(1*time.Second))

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)
}

func TestClientHardDisconnect(t *testing.T) {
	connect := connectPacket()
	connect.ClientID = "test"
	connect.CleanSession = false

	publish := packet.NewPublishPacket()
	publish.Message.Topic = "test"
	publish.Message.Payload = []byte("test")
	publish.Message.QOS = 1
	publish.ID = 1

	broker := flow.New().
		Receive(connect).
		Send(connackPacket()).
		Receive(publish).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	config := NewConfig("tcp://localhost:" + port)
	config.ClientID = "test"
	config.CleanSession = false

	connectFuture, err := c.Connect(config)
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	publishFuture, err := c.Publish("test", []byte("test"), 1, false)
	assert.NoError(t, err)
	assert.NotNil(t, publishFuture)

	err = c.Disconnect()
	assert.NoError(t, err)

	assert.Equal(t, future.ErrCanceled, publishFuture.Wait(1*time.Second))

	safeReceive(done)

	list, err := c.Session.AllPackets(session.Outgoing)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(list))
}

func TestClientDisconnectWithTimeout(t *testing.T) {
	publish := packet.NewPublishPacket()
	publish.Message.Topic = "test"
	publish.Message.Payload = []byte("test")
	publish.Message.QOS = 1
	publish.ID = 1

	puback := packet.NewPubackPacket()
	puback.ID = 1

	wait := func() {
		time.Sleep(100 * time.Millisecond)
	}

	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(publish).
		Run(wait).
		Send(puback).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	publishFuture, err := c.Publish("test", []byte("test"), 1, false)
	assert.NoError(t, err)
	assert.NotNil(t, publishFuture)

	err = c.Disconnect(10 * time.Second)
	assert.NoError(t, err)

	safeReceive(done)

	assert.NoError(t, publishFuture.Wait(1*time.Second))

	list, err := c.Session.AllPackets(session.Outgoing)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(list))
}

func TestClientClose(t *testing.T) {
	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = errorCallback(t)

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	err = c.Close()
	assert.NoError(t, err)

	safeReceive(done)
}

func TestClientInvalidPackets(t *testing.T) {
	c := New()

	// state not connecting
	err := c.processConnack(packet.NewConnackPacket())
	assert.NoError(t, err)

	c.state = clientConnecting

	// missing future
	err = c.processSuback(packet.NewSubackPacket())
	assert.NoError(t, err)

	// missing future
	err = c.processUnsuback(packet.NewUnsubackPacket())
	assert.NoError(t, err)

	// missing future
	err = c.processPubrel(0)
	assert.NoError(t, err)

	// missing future
	err = c.processPubackAndPubcomp(0)
	assert.NoError(t, err)
}

func TestClientSessionResumption(t *testing.T) {
	connect := connectPacket()
	connect.ClientID = "test"
	connect.CleanSession = false

	publish1 := packet.NewPublishPacket()
	publish1.Message.Topic = "test"
	publish1.Message.Payload = []byte("test")
	publish1.Message.QOS = 1
	publish1.ID = 1

	puback1 := packet.NewPubackPacket()
	puback1.ID = 1

	broker := flow.New().
		Receive(connect).
		Send(connackPacket()).
		Receive(publish1).
		Send(puback1).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Session.SavePacket(session.Outgoing, publish1)
	c.Session.NextID()
	c.Callback = errorCallback(t)

	config := NewConfig("tcp://localhost:" + port)
	config.ClientID = "test"
	config.CleanSession = false

	connectFuture, err := c.Connect(config)
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	time.Sleep(20 * time.Millisecond)

	err = c.Disconnect()
	assert.NoError(t, err)

	safeReceive(done)

	pkts, err := c.Session.AllPackets(session.Outgoing)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(pkts))
}

func TestClientUnexpectedClose(t *testing.T) {
	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Close()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.Nil(t, msg)
		assert.Error(t, err)
		close(wait)
		return nil
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	safeReceive(wait)
	safeReceive(done)
}

func TestClientConnackFutureCancellation(t *testing.T) {
	broker := flow.New().
		Receive(connectPacket()).
		Close()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.Nil(t, msg)
		assert.Error(t, err)
		close(wait)
		return nil
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.Equal(t, future.ErrCanceled, connectFuture.Wait(1*time.Second))

	safeReceive(wait)
	safeReceive(done)
}

func TestClientFutureCancellation(t *testing.T) {
	publish := packet.NewPublishPacket()
	publish.Message.Topic = "test"
	publish.Message.Payload = []byte("test")
	publish.Message.QOS = 1
	publish.ID = 1

	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(publish).
		Close()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.Nil(t, msg)
		assert.Error(t, err)
		return nil
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	publishFuture, err := c.Publish("test", []byte("test"), 1, false)
	assert.NoError(t, err)
	assert.Equal(t, future.ErrCanceled, publishFuture.Wait(1*time.Second))

	safeReceive(done)
}

func TestClientErrorCallback(t *testing.T) {
	publish := packet.NewPublishPacket()
	publish.Message.Topic = "test"
	publish.Message.Payload = []byte("test")
	publish.Message.QOS = 1
	publish.ID = 1

	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Send(publish).
		End()

	done, port := fakeBroker(t, broker)

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		assert.NotNil(t, msg)
		assert.NoError(t, err)
		return errors.New("some error")
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))
	assert.False(t, connectFuture.SessionPresent())
	assert.Equal(t, packet.ConnectionAccepted, connectFuture.ReturnCode())

	safeReceive(done)
}

func TestClientLogger(t *testing.T) {
	subscribe := packet.NewSubscribePacket()
	subscribe.Subscriptions = []packet.Subscription{{Topic: "test"}}
	subscribe.ID = 1

	suback := packet.NewSubackPacket()
	suback.ReturnCodes = []uint8{0}
	suback.ID = 1

	publish := packet.NewPublishPacket()
	publish.Message.Topic = "test"
	publish.Message.Payload = []byte("test")

	broker := flow.New().
		Receive(connectPacket()).
		Send(connackPacket()).
		Receive(subscribe).
		Send(suback).
		Receive(publish).
		Send(publish).
		Receive(disconnectPacket()).
		End()

	done, port := fakeBroker(t, broker)

	wait := make(chan struct{})

	c := New()
	c.Callback = func(msg *packet.Message, err error) error {
		close(wait)
		return nil
	}

	var counter uint32
	c.Logger = func(msg string) {
		atomic.AddUint32(&counter, 1)
	}

	connectFuture, err := c.Connect(NewConfig("tcp://localhost:" + port))
	assert.NoError(t, err)
	assert.NoError(t, connectFuture.Wait(1*time.Second))

	subscribeFuture, err := c.Subscribe("test", 0)
	assert.NoError(t, err)
	assert.NoError(t, subscribeFuture.Wait(1*time.Second))

	publishFuture, err := c.Publish("test", []byte("test"), 0, false)
	assert.NoError(t, err)
	assert.NoError(t, publishFuture.Wait(1*time.Second))

	safeReceive(wait)

	assert.NoError(t, c.Disconnect())

	safeReceive(done)

	assert.Equal(t, uint32(8), counter)
}

func BenchmarkClientPublish(b *testing.B) {
	c := New()

	connectFuture, err := c.Connect(NewConfig("mqtt://0.0.0.0"))
	if err != nil {
		panic(err)
	}

	err = connectFuture.Wait(1 * time.Second)
	if err != nil {
		panic(err)
	}

	for i := 0; i < b.N; i++ {
		_, err := c.Publish("test", []byte("test"), 0, false)
		if err != nil {
			panic(err)
		}
	}

	err = c.Disconnect()
	if err != nil {
		panic(err)
	}
}
