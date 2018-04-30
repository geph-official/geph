package common

// AllowedPorts lists all the ports that Geph is allowed to tunnel.
var AllowedPorts = map[int]bool{
	20: true, 21: true, // FTP
	22:    true, // SSH
	53:    true, // DNS
	80:    true, // HTTP
	110:   true, // POP3
	143:   true, // IMAP
	220:   true, // IMAP3
	443:   true, // HTTPS
	531:   true, // AIM
	873:   true, // rsync
	989:   true, // FTPS
	990:   true, // FTPS
	991:   true, // NAS Usenet
	992:   true, // TELNETS
	993:   true, // IMAPS
	995:   true, // POP3S
	1194:  true, // OpenVPN
	1293:  true, // IPSec
	3690:  true, // SVN
	4321:  true, // RWHOIS
	5222:  true, // XMPP
	5322:  true, // XMPP
	5228:  true, // Android
	8080:  true, // HTTP alternative
	8332:  true, // Bitcoin
	8333:  true, // Bitcoin
	8888:  true, // Freenet etc
	9418:  true, // git
	11371: true, // OpenPGP hkp
	19294: true, // Google Voice
	50002: true, // Electrum bitcoin
	64738: true, // Mumble
}
