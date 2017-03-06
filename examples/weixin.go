package main
import (
	"bytes"
	"fmt"
	"log"
	"github.com/wowotech/gatt"
	"github.com/wowotech/gatt/examples/option"
)

var mac [6]byte

// cmdReadBDAddr implements cmd.CmdParam for demostrating LnxSendHCIRawCommand()
type cmdReadBDAddr struct{}

func (c cmdReadBDAddr) Marshal(b []byte) {}
func (c cmdReadBDAddr) Opcode() int      { return 0x1009 }
func (c cmdReadBDAddr) Len() int         { return 0 }

// Get bdaddr with LnxSendHCIRawCommand() for demo purpose
func bdaddr(d gatt.Device) {
	rsp := bytes.NewBuffer(nil)
	if err := d.Option(gatt.LnxSendHCIRawCommand(&cmdReadBDAddr{}, rsp)); err != nil {
		fmt.Printf("Failed to send HCI raw command, err: %s", err)
	}
	b := rsp.Bytes()
	if b[0] != 0 {
		fmt.Printf("Failed to get bdaddr with HCI Raw command, status: %d", b[0])
	}
	log.Printf("BD Addr: %02X:%02X:%02X:%02X:%02X:%02X", b[6], b[5], b[4], b[3], b[2], b[1])

	mac[0] = b[6]
	mac[1] = b[5]
	mac[2] = b[4]
	mac[3] = b[3]
	mac[4] = b[2]
	mac[5] = b[1]
}

func main() {

	// steps little endian
	// 01(steps) 10 27 00(0x002710 = 10000)
	// http://iot.weixin.qq.com/wiki/new/index.html?page=4-3
	steps := []byte{ 0x01, 0x5c, 0x74, 0x01 }

	const (
		flagLimitedDiscoverable	= 0x01	// LE Limited Discoverable Mode
		flagGeneralDiscoverable	= 0x02	// LE General Discoverable Mode
		flagLEOnly		= 0x04	// BR/EDR Not Supported. Bit 37 of LMP Feature Mask Definitions (Page 0)
		flagBothController	= 0x08	// Simultaneous LE and BR/EDR to Same Device Capable (Controller).
		flagBothHost		= 0x10	// Simultaneous LE and BR/EDR to Same Device Capable (Host). 
	)

	const (
		wxServiceUuid		= 0xFEE7// Weixin service UUID

		wxChWriteUuid		= 0xFEC7// Weixin Write character UUID
		wxChIndicateUuid	= 0xFEC8// Weixin Indicate character UUID
		wxChReadUuid		= 0xFEC9// Weixin Read character UUID

		wxChPedometerUuid	= 0xFEA1// Weixin Pedometer character UUID
		wxChTargetUuid		= 0xFEA2// Weixin Target character UUID
	)


	d, err := gatt.NewDevice(option.DefaultServerOptions...)
	if err != nil {
		log.Fatalf("Failed to open device, err: %s", err)
	}

	// Register optional handlers.
	d.Handle(
		gatt.CentralConnected(func(c gatt.Central) { fmt.Println("Connect: ", c.ID()) }),
		gatt.CentralDisconnected(func(c gatt.Central) { fmt.Println("Disconnect: ", c.ID()) }),
	)

	// A mandatory handler for monitoring device state.
	onStateChanged := func(d gatt.Device, s gatt.State) {
		fmt.Printf("State: %s\n", s)
		switch s {
		case gatt.StatePoweredOn:
			bdaddr(d)

			// get service
			s0 := gatt.NewService(gatt.UUID16(wxServiceUuid))

			// add pedometer character
			c0 := s0.AddCharacteristic(gatt.UUID16(wxChPedometerUuid))
			c0.HandleReadFunc(
				func(rsp gatt.ResponseWriter, req *gatt.ReadRequest) {
					log.Println("Read: pedometer character")
					rsp.Write(steps)
				})
			c0.HandleNotifyFunc(
				func(r gatt.Request, n gatt.Notifier) {
					go func() {
						n.Write(steps)
						log.Printf("Indicate: pedometer character")
					}()
				})

			// add target character
			c1 := s0.AddCharacteristic(gatt.UUID16(wxChTargetUuid))
			c1.HandleReadFunc(
				func(rsp gatt.ResponseWriter, req *gatt.ReadRequest) {
					log.Println("Read: target character")
					rsp.Write(steps)
				})
			c1.HandleNotifyFunc(
				func(r gatt.Request, n gatt.Notifier) {
					go func() {
						n.Write(steps)
						log.Printf("Indicate: target character")
					}()
				})
			c1.HandleWriteFunc(
				func(r gatt.Request, data []byte) (status byte) {
					log.Println("Wrote target character:", string(data))
					return gatt.StatusSuccess
				})

			// add read character
			c2 := s0.AddCharacteristic(gatt.UUID16(wxChReadUuid))
			c2.HandleReadFunc(
				func(rsp gatt.ResponseWriter, req *gatt.ReadRequest) {
					log.Println("Read: read character")
					rsp.Write(mac[:])
				})

			// add service
			d.AddService(s0)

			// Advertise device name and service's UUIDs.
			a := &gatt.AdvPacket{}
			a.AppendFlags(flagGeneralDiscoverable | flagLEOnly)
			a.AppendUUIDFit([]gatt.UUID{s0.UUID()})
			a.AppendName("WeixinBLE")

			// company id and data, MAC Address
			// https://www.bluetooth.com/specifications/assigned-numbers/company-identifiers
			a.AppendManufacturerData(0x2333, mac[:])
			d.Advertise(a)

		default:
		}
	}

	d.Init(onStateChanged)
	select {}
}
