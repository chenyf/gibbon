package util

func GetMac() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Infof("get mac addres failed:%v\r\n", err)
	}
	var macAddr string
	var mac string
	for _, inter := range interfaces {
		macAddr = inter.HardwareAddr.String()
		if macAddr != "" {
			mac = strings.Replace(macAddr, ":", "", -1)
			log.Infof("mac address:%s", mac)
		}
	}
	return mac, err
}

func myread(conn *net.TCPConn, buf []byte) int {
	n, err := io.ReadFull(conn, buf)
	if err != nil {
		if e, ok := err.(*net.OpError); ok && e.Timeout() {
			return n
		}
		log.Debugf("%p: readfull failed (%v)", conn, err)
		return -1
	}
	return n
}

