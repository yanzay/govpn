package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"

	"golang.org/x/crypto/ssh/terminal"
)

var (
	user     = flag.String("user", "", "User name")
	pk       = flag.String("pk", defaultKeyPath(), "Private key file")
	host     = flag.String("host", "", "Host")
	port     = flag.Int("port", 22, "Port")
	vpnuser  = flag.String("vpn-user", "", "VPN user name")
	password string
)

func defaultKeyPath() string {
	home := os.Getenv("HOME")
	if len(home) > 0 {
		return path.Join(home, ".ssh/id_rsa")
	}
	return ""
}

func main() {
	flag.Parse()
	client, err := NewSSHClient(*pk, *user, *host, *port)
	if err != nil {
		panic(err)
	}
	password, err = getPassword()
	if err != nil {
		panic(err)
	}
	_, err = client.Command("apt install docker.io -y")
	if err != nil {
		panic(err)
	}
	_, err = client.Commandf("docker run -v ovpn-data:/etc/openvpn --rm kylemanna/openvpn ovpn_genconfig -u udp://%s", *host)
	if err != nil {
		panic(err)
	}
	err = client.InteractiveCommand("docker run -v ovpn-data:/etc/openvpn --rm -i kylemanna/openvpn ovpn_initpki")
	if err != nil {
		panic(err)
	}
	_, err = client.Command("docker stop openvpn || true && docker rm openvpn || true")
	if err != nil {
		panic(err)
	}
	_, err = client.Command("docker run -v ovpn-data:/etc/openvpn -d --name openvpn -p 1194:1194/udp --cap-add=NET_ADMIN kylemanna/openvpn")
	if err != nil {
		panic(err)
	}
	err = client.InteractiveCommandf("docker run -v ovpn-data:/etc/openvpn --rm -i kylemanna/openvpn easyrsa build-client-full %s nopass", *vpnuser)
	if err != nil {
		panic(err)
	}
	out, err := client.Commandf("docker run -v ovpn-data:/etc/openvpn --rm kylemanna/openvpn ovpn_getclient %s", *vpnuser)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(*vpnuser + ".ovpn")
	if err != nil {
		panic(err)
	}
	_, err = io.Copy(f, out)
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}
}

func getPassword() (string, error) {
	fmt.Print("Enter pass phrase: ")
	pass, err := terminal.ReadPassword(0)
	if err != nil {
		return "", err
	}
	fmt.Println()
	return string(pass), nil
}
