package models

var tcpServiceHints = map[int]string{
	20:    "FTP data",
	21:    "FTP",
	22:    "SSH",
	23:    "Telnet",
	25:    "SMTP",
	53:    "DNS",
	80:    "HTTP",
	110:   "POP3",
	111:   "rpcbind",
	135:   "Microsoft RPC",
	139:   "NetBIOS session",
	143:   "IMAP",
	389:   "LDAP",
	443:   "HTTPS",
	445:   "SMB",
	465:   "SMTPS",
	554:   "RTSP",
	587:   "SMTP submission",
	631:   "IPP",
	636:   "LDAPS",
	853:   "DNS over TLS",
	993:   "IMAPS",
	995:   "POP3S",
	1433:  "Microsoft SQL Server",
	1521:  "Oracle Database",
	1883:  "MQTT",
	2049:  "NFS",
	2375:  "Docker API",
	2376:  "Docker API TLS",
	3306:  "MySQL",
	3389:  "RDP",
	5432:  "PostgreSQL",
	5900:  "VNC",
	6379:  "Redis",
	8080:  "HTTP alternate",
	8443:  "HTTPS alternate",
	8883:  "MQTT TLS",
	27017: "MongoDB",
	32400: "Plex Media Server",
}

// ServiceHintForPort returns a conservative well-known-port label.
// It is a hint only; LANnventory does not claim protocol fingerprinting.
func ServiceHintForPort(protocol string, port int) string {
	if protocol != string(ServiceProtocolTCP) {
		return ""
	}
	return tcpServiceHints[port]
}
