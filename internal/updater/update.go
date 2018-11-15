package updater

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

const (
	query         = "SELECT login, password, addressFrom, deviceState, deviceOptions FROM inet_serv_14 WHERE deviceId = ?;"
	defaultShaper = "vpn_2Mb"
	stateActive   = 1
)

var (
	shapers = map[string]string{
		"8":  "vpn_1Mb",
		"20": "vpn_10Mb",
		"11": "vpn_1,5Mb",
		"26": "vpn_12Mb",
		"28": "vpn_15Mb",
		"9":  "vpn_2Mb",
		"25": "vpn_20Mb",
		"12": "vpn_2,5Mb",
		"10": "vpn_3Mb",
		"13": "vpn_4Mb",
		"14": "vpn_5Mb",
		"30": "vpn_50Mb",
		"15": "vpn_6Mb",
		"16": "vpn_7Mb",
		"17": "vpn_8Mb",
		"18": "vpn_9Mb",
	}
)

type service struct {
	login    string
	password string
	address  net.IP
	shaper   string
	state    int
}

// Updater updates records for given device.
type Updater interface {
	Update(context context.Context, deviceID int) error
}

type basic struct {
	db     *sql.DB
	client *ssh.Client
}

// New return ready to use Updater.
func New(client *ssh.Client, db *sql.DB) Updater {
	b := basic{
		client: client,
		db:     db,
	}

	return &b
}

func (b *basic) Update(context context.Context, deviceID int) error {
	stmt, err := b.db.Prepare(query)
	if err != nil {
		return errors.Wrap(err, "prepare stmt")
	}

	rows, err := stmt.QueryContext(context, deviceID)
	if err != nil {
		return errors.Wrapf(err, "query context with device: %d", deviceID)
	}

	var services []service

	for rows.Next() {
		s := service{
			shaper: defaultShaper,
		}

		var deviceOption string
		if err := rows.Scan(&s.login, &s.password, &s.address, &s.state, &deviceOption); err != nil {
			return errors.Wrap(err, "scan failed")
		}

		if shpr, ok := shapers[deviceOption]; ok {
			s.shaper = shpr
		}

		services = append(services, s)
	}

	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "rows contains error")
	}

	if err := b.exec("/ppp secret remove [/ppp secret find]"); err != nil {
		return errors.Wrap(err, "erase previous")
	}

	buf := new(bytes.Buffer)

	for _, s := range services {
		if s.state == stateActive {
			cmd := b.buildCmd(&s)
			if _, err := buf.WriteString(cmd); err != nil {
				return errors.Wrap(err, "write to buffer")
			}
		}
	}

	if err := b.exec(buf.String()); err != nil {
		return errors.Wrapf(err, "buffer error")
	}

	return nil
}

func (b *basic) buildCmd(s *service) string {
	return fmt.Sprintf("/ppp secret add local-address=10.0.0.0 name=%s password=%s remote-address=%s disabled=yes profile=%s\n", s.login, s.password, s.address, s.shaper)
}

func (b *basic) exec(cmd string) error {
	session, err := b.client.NewSession()

	if err != nil {
		return errors.Wrap(err, "new session")
	}
	defer session.Close()

	if err := session.Run(cmd); err != nil {
		return errors.Wrap(err, "run cmd")
	}

	return nil
}
