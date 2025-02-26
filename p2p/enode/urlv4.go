// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package enode

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/crypto/cryptobase"
	"github.com/QuantumCoinProject/qc/crypto/signaturealgorithm"
	"net"
	"net/url"
	"regexp"
	"strconv"

	"github.com/QuantumCoinProject/qc/crypto"
	"github.com/QuantumCoinProject/qc/p2p/enr"
)

var (
	incompleteNodeURL = regexp.MustCompile("(?i)^(?:enode://)?([0-9a-f]+)$")
	lookupIPFunc      = net.LookupIP
)

// MustParseV4 parses a node URL. It panics if the URL is not valid.
func MustParseV4(rawurl string) *Node {
	n, err := ParseV4(rawurl)
	if err != nil {
		panic("invalid node URL: " + err.Error())
	}
	return n
}

// ParseV4 parses a node URL.
//
// There are two basic forms of node URLs:
//
//   - incomplete nodes, which only have the public key (node ID)
//   - complete nodes, which contain the public key and IP/Port information
//
// For incomplete nodes, the designator must look like one of these
//
//	enode://<hex node id>
//	<hex node id>
//
// For complete nodes, the node ID is encoded in the username portion
// of the URL, separated from the host by an @ sign. The hostname can
// only be given as an IP address or using DNS domain name.
// The port in the host name section is the TCP listening port. If the
// TCP and UDP (discovery) ports differ, the UDP port is specified as
// query parameter "discport".
//
// In the following example, the node URL describes
// a node with IP address 10.3.58.6, TCP listening port 30303
// and UDP discovery port 30301.
//
//	enode://<hex node id>@10.3.58.6:30303?discport=30301
func ParseV4(rawurl string) (*Node, error) {
	if m := incompleteNodeURL.FindStringSubmatch(rawurl); m != nil {
		id, err := parsePubkey(m[1])
		if err != nil {
			return nil, fmt.Errorf("invalid public key (%v)", err)
		}
		return NewV4(id, nil, 0), nil
	}
	return parseComplete(rawurl)
}

// NewV4 creates a node from discovery v4 node information. The record
// contained in the node has a zero-length signature.
func NewV4(pubkey *signaturealgorithm.PublicKey, ip net.IP, tcp int) *Node {
	var r enr.Record
	if len(ip) > 0 {
		r.Set(enr.IP(ip))
	}
	if tcp != 0 {
		r.Set(enr.TCP(tcp))
	}
	signV4Compat(&r, pubkey)
	n, err := New(v4CompatID{}, &r)
	if err != nil {
		panic(err)
	}
	return n
}

// isNewV4 returns true for nodes created by NewV4.
func isNewV4(n *Node) bool {
	var k s256raw
	return n.r.IdentityScheme() == "" && n.r.Load(&k) == nil && len(n.r.Signature()) == 0
}

func parseComplete(rawurl string) (*Node, error) {
	var (
		id      *signaturealgorithm.PublicKey
		tcpPort uint64
	)
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "enode" {
		return nil, errors.New("invalid URL scheme, want \"enode\"")
	}
	// Parse the Node ID from the user portion.
	if u.User == nil {
		return nil, errors.New("does not contain node ID")
	}
	if id, err = parsePubkey(u.User.String()); err != nil {
		return nil, fmt.Errorf("invalid public key (%v)", err)
	}
	// Parse the IP address.
	ip := net.ParseIP(u.Hostname())
	if ip == nil {
		ips, err := lookupIPFunc(u.Hostname())
		if err != nil {
			return nil, err
		}
		ip = ips[0]
	}
	// Ensure the IP is 4 bytes long for IPv4 addresses.
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	// Parse the port numbers.
	if tcpPort, err = strconv.ParseUint(u.Port(), 10, 16); err != nil {
		return nil, errors.New("invalid port")
	}

	return NewV4(id, ip, int(tcpPort)), nil
}

// parsePubkey parses a hex-encoded public key.
func parsePubkey(in string) (*signaturealgorithm.PublicKey, error) {
	b, err := hex.DecodeString(in)
	if err != nil {
		return nil, err
	}

	return cryptobase.SigAlg.DeserializePublicKey(b)
}

func (n *Node) URLv4() string {
	var (
		scheme enr.ID
		nodeid string
		key    signaturealgorithm.PublicKey
	)
	n.Load(&scheme)
	n.Load((*PqPubKey)(&key))

	switch {
	case scheme == "v4" || len(key.PubData) > 0:
		data, err := cryptobase.SigAlg.SerializePublicKey(&key)
		if err != nil {

			return ""
		}
		nodeid = fmt.Sprintf("%x", data)
	default:
		nodeid = fmt.Sprintf("%s.%x", scheme, n.id[:])
	}
	u := url.URL{Scheme: "enode"}
	if n.Incomplete() {
		u.Host = nodeid
	} else {
		addr := net.TCPAddr{IP: n.IP(), Port: n.TCP()}
		u.User = url.User(nodeid)
		u.Host = addr.String()
	}
	return u.String()
}

// PubkeyToIDV4 derives the v4 node address from the given public key.
func PubkeyToIDV4(key *signaturealgorithm.PublicKey) ID {
	e := make([]byte, cryptobase.SigAlg.PublicKeyLength())
	pubBytes, err := cryptobase.SigAlg.SerializePublicKey(key)
	if err != nil {
		panic(err)
	}
	copy(e, pubBytes)
	return ID(crypto.Keccak256Hash(e))
}
