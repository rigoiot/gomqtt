package packet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnsubscribeInterface(t *testing.T) {
	pkt := NewUnsubscribePacket()
	pkt.Topics = []string{"foo", "bar"}

	assert.Equal(t, pkt.Type(), UNSUBSCRIBE)
	assert.Equal(t, "<UnsubscribePacket Topics=[\"foo\", \"bar\"]>", pkt.String())
}

func TestUnsubscribePacketDecode(t *testing.T) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		32,
		0, // packet ID MSB
		7, // packet ID LSB
		0, // topic name MSB
		6, // topic name LSB
		'g', 'o', 'm', 'q', 't', 't',
		0, // topic name MSB
		8, // topic name LSB
		'/', 'a', '/', 'b', '/', '#', '/', 'c',
		0,  // topic name MSB
		10, // topic name LSB
		'/', 'a', '/', 'b', '/', '#', '/', 'c', 'd', 'd',
	}

	pkt := NewUnsubscribePacket()
	n, err := pkt.Decode(pktBytes)

	assert.NoError(t, err)
	assert.Equal(t, len(pktBytes), n)
	assert.Equal(t, 3, len(pkt.Topics))
	assert.Equal(t, "gomqtt", pkt.Topics[0])
	assert.Equal(t, "/a/b/#/c", pkt.Topics[1])
	assert.Equal(t, "/a/b/#/cdd", pkt.Topics[2])
}

func TestUnsubscribePacketDecodeError1(t *testing.T) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		2,
		0, // packet ID MSB
		7, // packet ID LSB
		// empty topic list
	}

	pkt := NewUnsubscribePacket()
	_, err := pkt.Decode(pktBytes)

	assert.Error(t, err)
}

func TestUnsubscribePacketDecodeError2(t *testing.T) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		6, // < wrong remaining length
		0, // packet ID MSB
		7, // packet ID LSB
	}

	pkt := NewUnsubscribePacket()
	_, err := pkt.Decode(pktBytes)

	assert.Error(t, err)
}

func TestUnsubscribePacketDecodeError3(t *testing.T) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		0,
		// missing packet id
	}

	pkt := NewUnsubscribePacket()
	_, err := pkt.Decode(pktBytes)

	assert.Error(t, err)
}

func TestUnsubscribePacketDecodeError4(t *testing.T) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		10,
		0, // packet ID MSB
		7, // packet ID LSB
		0, // topic name MSB
		9, // topic name LSB < wrong size
		'g', 'o', 'm', 'q', 't', 't',
	}

	pkt := NewUnsubscribePacket()
	_, err := pkt.Decode(pktBytes)

	assert.Error(t, err)
}

func TestUnsubscribePacketDecodeError5(t *testing.T) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		10,
		0, // packet ID MSB
		0, // packet ID LSB < zero packet id
		0, // topic name MSB
		6, // topic name LSB
		'g', 'o', 'm', 'q', 't', 't',
	}

	pkt := NewUnsubscribePacket()
	_, err := pkt.Decode(pktBytes)

	assert.Error(t, err)
}

func TestUnsubscribePacketEncode(t *testing.T) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		32,
		0, // packet ID MSB
		7, // packet ID LSB
		0, // topic name MSB
		6, // topic name LSB
		'g', 'o', 'm', 'q', 't', 't',
		0, // topic name MSB
		8, // topic name LSB
		'/', 'a', '/', 'b', '/', '#', '/', 'c',
		0,  // topic name MSB
		10, // topic name LSB
		'/', 'a', '/', 'b', '/', '#', '/', 'c', 'd', 'd',
	}

	pkt := NewUnsubscribePacket()
	pkt.ID = 7
	pkt.Topics = []string{
		"gomqtt",
		"/a/b/#/c",
		"/a/b/#/cdd",
	}

	dst := make([]byte, 100)
	n, err := pkt.Encode(dst)

	assert.NoError(t, err)
	assert.Equal(t, len(pktBytes), n)
	assert.Equal(t, pktBytes, dst[:n])
}

func TestUnsubscribePacketEncodeError1(t *testing.T) {
	pkt := NewUnsubscribePacket()
	pkt.ID = 7
	pkt.Topics = []string{"gomqtt"}

	dst := make([]byte, 1) // < too small
	n, err := pkt.Encode(dst)

	assert.Error(t, err)
	assert.Equal(t, 0, n)
}

func TestUnsubscribePacketEncodeError2(t *testing.T) {
	pkt := NewUnsubscribePacket()
	pkt.ID = 7
	pkt.Topics = []string{string(make([]byte, 65536))}

	dst := make([]byte, pkt.Len())
	n, err := pkt.Encode(dst)

	assert.Error(t, err)
	assert.Equal(t, 6, n)
}

func TestUnsubscribePacketEncodeError3(t *testing.T) {
	pkt := NewUnsubscribePacket()
	pkt.ID = 0 // < zero packet id

	dst := make([]byte, pkt.Len())
	n, err := pkt.Encode(dst)

	assert.Error(t, err)
	assert.Equal(t, 0, n)
}

func TestUnsubscribeEqualDecodeEncode(t *testing.T) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		32,
		0, // packet ID MSB
		7, // packet ID LSB
		0, // topic name MSB
		6, // topic name LSB
		'g', 'o', 'm', 'q', 't', 't',
		0, // topic name MSB
		8, // topic name LSB
		'/', 'a', '/', 'b', '/', '#', '/', 'c',
		0,  // topic name MSB
		10, // topic name LSB
		'/', 'a', '/', 'b', '/', '#', '/', 'c', 'd', 'd',
	}

	pkt := NewUnsubscribePacket()
	n, err := pkt.Decode(pktBytes)

	assert.NoError(t, err)
	assert.Equal(t, len(pktBytes), n)

	dst := make([]byte, 100)
	n2, err := pkt.Encode(dst)

	assert.NoError(t, err)
	assert.Equal(t, len(pktBytes), n2)
	assert.Equal(t, pktBytes, dst[:n2])

	n3, err := pkt.Decode(dst)

	assert.NoError(t, err)
	assert.Equal(t, len(pktBytes), n3)
}

func BenchmarkUnsubscribeEncode(b *testing.B) {
	pkt := NewUnsubscribePacket()
	pkt.ID = 1
	pkt.Topics = []string{"t"}

	buf := make([]byte, pkt.Len())

	for i := 0; i < b.N; i++ {
		_, err := pkt.Encode(buf)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkUnsubscribeDecode(b *testing.B) {
	pktBytes := []byte{
		byte(UNSUBSCRIBE<<4) | 2,
		5,
		0, // packet ID MSB
		1, // packet ID LSB
		0, // topic name MSB
		1, // topic name LSB
		't',
	}

	pkt := NewUnsubscribePacket()

	for i := 0; i < b.N; i++ {
		_, err := pkt.Decode(pktBytes)
		if err != nil {
			panic(err)
		}
	}
}
