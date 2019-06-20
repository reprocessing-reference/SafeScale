/*
 * Copyright 2018-2019, CS Systemes d'Information, http://www.c-s.fr
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package system

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/CS-SI/SafeScale/lib/utils"
	"github.com/CS-SI/SafeScale/lib/utils/retry"
	"golang.org/x/crypto/ssh"
)

const sshOptions = "-q -oIdentitiesOnly=yes -oStrictHostKeyChecking=no -oUserKnownHostsFile=/dev/null -oPubkeyAuthentication=yes -oPasswordAuthentication=no"

var (
	sshErrorMap = map[int]string{
		1:  "Malformed configuration or invalid cli options",
		2:  "Connection failed",
		65: "Host not allowed to connect",
		66: "General error in ssh protocol",
		67: "Key exchange failed",
		69: "MAC error",
		70: "Compression error",
		71: "Service not available",
		72: "Protocol version not supported",
		73: "Host key not verifiable",
		74: "Connection failed",
		75: "Disconnected by application",
		76: "Too many connections",
		77: "Authentication cancelled by user",
		78: "No more authentication methods available",
		79: "Invalid user name",
	}
	scpErrorMap = map[int]string{
		1:  "General error in file copy",
		2:  "Destination is not directory, but it should be",
		3:  "Maximum symlink level exceeded",
		4:  "Connecting to host failed",
		5:  "Connection broken",
		6:  "File does not exist",
		7:  "No permission to access file",
		8:  "General error in sftp protocol",
		9:  "File transfer protocol mismatch",
		10: "No file matches a given criteria",
		65: "Host not allowed to connect",
		66: "General error in ssh protocol",
		67: "Key exchange failed",
		69: "MAC error",
		70: "Compression error",
		71: "Service not available",
		72: "Protocol version not supported",
		73: "Host key not verifiable",
		74: "Connection failed",
		75: "Disconnected by application",
		76: "Too many connections",
		77: "Authentication cancelled by user",
		78: "No more authentication methods available",
		79: "Invalid user name",
	}
)

// IsSSHRetryable tells if the retcode of a ssh command may be retried
func IsSSHRetryable(code int) bool {
	if code == 2 || code == 4 || code == 5 || code == 66 || code == 67 || code == 70 || code == 74 || code == 75 || code == 76 {
		return true
	}
	return false

}

// IsSCPRetryable tells if the retcode of a scp command may be retried
func IsSCPRetryable(code int) bool {
	if code == 4 || code == 5 || code == 66 || code == 67 || code == 70 || code == 74 || code == 75 || code == 76 {
		return true
	}
	return false
}

// SSHConfig helper to manage ssh session
type SSHConfig struct {
	User          string
	Host          string
	PrivateKey    string
	Port          int
	LocalPort     int
	GatewayConfig *SSHConfig
	cmdTpl        string
}

// SSHTunnel a SSH tunnel
type SSHTunnel struct {
	port      int
	cmd       *exec.Cmd
	cmdString string
	keyFile   *os.File
}

// SSHErrorString returns if possible the string corresponding to SSH execution
func SSHErrorString(retcode int) string {
	if msg, ok := sshErrorMap[retcode]; ok {
		return msg
	}
	return "Unqualified error"
}

// SCPErrorString returns if possible the string corresponding to SCP execution
func SCPErrorString(retcode int) string {
	if msg, ok := scpErrorMap[retcode]; ok {
		return msg
	}
	return "Unqualified error"
}

// Close closes ssh tunnel
func (tunnel *SSHTunnel) Close() error {
	defer func() {
		lazyErr := utils.LazyRemove(tunnel.keyFile.Name())
		if lazyErr != nil {
			log.Error(lazyErr)
		}
	}()

	// Kills the process of the tunnel
	err := tunnel.cmd.Process.Kill()
	if err != nil {
		log.Errorf("tunnel.cmd.Process.Kill() failed: %s\n", reflect.TypeOf(err).String())
		return fmt.Errorf("Unable to close tunnel :%s", err.Error())
	}
	// Kills remaining processes if there are some
	bytes, err := exec.Command("pgrep", "-f", tunnel.cmdString).Output()
	if err == nil {
		portStr := strings.Trim(string(bytes), "\n")
		_, err = strconv.Atoi(portStr)
		if err == nil {
			err = exec.Command("kill", "-9", portStr).Run()
			if err != nil {
				log.Errorf("kill -9 failed: %s\n", reflect.TypeOf(err).String())
				return fmt.Errorf("Unable to close tunnel :%s", err.Error())
			}
		}
	}
	return nil
}

// GetFreePort get a free port
func getFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	defer func() {
		clErr := listener.Close()
		if clErr != nil {
			log.Error(clErr)
		}
	}()
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	return port, nil
}

// CreateTempFileFromString creates a tempory file containing 'content'
func CreateTempFileFromString(content string, filemode os.FileMode) (*os.File, error) {
	defaultTmpDir := "/tmp"
	if runtime.GOOS == "windows" {
		defaultTmpDir = ""
	}

	f, err := ioutil.TempFile(defaultTmpDir, "") // TODO Windows friendly
	if err != nil {
		return nil, err
	}
	_, err = f.WriteString(content)
	if err != nil {
		log.Warnf("Error writing string: %v", err)
	}

	err = f.Chmod(filemode)
	if err != nil {
		log.Warnf("Error changing directory: %v", err)
	}

	err = f.Close()
	if err != nil {
		log.Warnf("Error closing file: %v", err)
	}

	return f, nil
}

func isTunnelReady(port int) bool {
	// Try to create a server with the port
	server, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return true
	}
	err = server.Close()
	if err != nil {
		log.Warnf("Error closing server: %v", err)
	}
	return false

}

// buildTunnel create SSH from local host to remote host through gateway
// if localPort is set to 0 then it's  automatically choosed
func buildTunnel(cfg *SSHConfig) (*SSHTunnel, error) {
	f, err := CreateTempFileFromString(cfg.GatewayConfig.PrivateKey, 0400)
	if err != nil {
		return nil, err
	}
	localPort := cfg.LocalPort
	if localPort == 0 {
		localPort, err = getFreePort()
		if err != nil {
			return nil, err
		}
	}

	options := sshOptions + " -oServerAliveInterval=60"
	cmdString := fmt.Sprintf("ssh -i %s -NL 127.0.0.1:%d:%s:%d %s@%s %s -p %d",
		f.Name(),
		localPort,
		cfg.Host,
		cfg.Port,
		cfg.GatewayConfig.User,
		cfg.GatewayConfig.Host,
		options,
		cfg.GatewayConfig.Port,
	)
	cmd := exec.Command("sh", "-c", cmdString)
	err = cmd.Start()
	//	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	for nbiter := 0; !isTunnelReady(localPort) && nbiter < 100; nbiter++ {
		time.Sleep(10 * time.Millisecond)
	}
	return &SSHTunnel{
		port:      localPort,
		cmd:       cmd,
		cmdString: cmdString,
		keyFile:   f,
	}, nil
}

// SSHCommand defines a SSH command
type SSHCommand struct {
	cmd     *exec.Cmd
	tunnels []*SSHTunnel
	keyFile *os.File
}

func (sc *SSHCommand) closeTunneling() error {
	var err error
	for _, t := range sc.tunnels {
		err = t.Close()
	}
	sc.tunnels = []*SSHTunnel{}

	// Tunnels are imbricated only last error is significant
	if err != nil {
		log.Errorf("closeTunneling: %s\n", reflect.TypeOf(err).String())
	}
	return err
}

// Wait waits for the command to exit and waits for any copying to stdin or copying from stdout or stderr to complete.
// The command must have been started by Start.
// The returned error is nil if the command runs, has no problems copying stdin, stdout, and stderr, and exits with a zero exit status.
// If the command fails to run or doesn't complete successfully, the error is of type *ExitError. Other error types may be returned for I/O problems.
// Wait also waits for the I/O loop copying from c.Stdin into the process's standard input to complete.
// Wait releases any resources associated with the cmd.
func (sc *SSHCommand) Wait() error {
	err := sc.cmd.Wait()
	nerr := sc.cleanup()
	if err != nil {
		return err
	}
	if nerr != nil {
		log.Warnf("Error waiting for command cleanup: %v", nerr)
	}
	return nerr
}

// Kill kills SSHCommand process and releases any resources associated with the SSHCommand.
func (sc *SSHCommand) Kill() error {
	return sc.cmd.Process.Kill()
}

// StdoutPipe returns a pipe that will be connected to the command's standard output when the command starts.
// Wait will close the pipe after seeing the command exit, so most callers need not close the pipe themselves; however, an implication is that it is incorrect to call Wait before all reads from the pipe have completed.
// For the same reason, it is incorrect to call Run when using StdoutPipe.
func (sc *SSHCommand) StdoutPipe() (io.ReadCloser, error) {
	return sc.cmd.StdoutPipe()
}

// StderrPipe returns a pipe that will be connected to the command's standard error when the command starts.
// Wait will close the pipe after seeing the command exit, so most callers need not close the pipe themselves; however, an implication is that it is incorrect to call Wait before all reads from the pipe have completed. For the same reason, it is incorrect to use Run when using StderrPipe.
func (sc *SSHCommand) StderrPipe() (io.ReadCloser, error) {
	return sc.cmd.StderrPipe()
}

// StdinPipe returns a pipe that will be connected to the command's standard input when the command starts.
// The pipe will be closed automatically after Wait sees the command exit.
// A caller need only call Close to force the pipe to close sooner.
// For example, if the command being run will not exit until standard input is closed, the caller must close the pipe.
func (sc *SSHCommand) StdinPipe() (io.WriteCloser, error) {
	return sc.cmd.StdinPipe()
}

// Output runs the command and returns its standard output.
// Any returned error will usually be of type *ExitError.
// If c.Stderr was nil, Output populates ExitError.Stderr.
func (sc *SSHCommand) Output() ([]byte, error) {
	content, err := sc.cmd.Output()
	nerr := sc.cleanup()
	if err != nil {
		return nil, err
	}
	if nerr != nil {
		log.Warnf("Error waiting for command cleanup: %v", nerr)
	}
	return content, err
}

// CombinedOutput runs the command and returns its combined standard
// output and standard error.
func (sc *SSHCommand) CombinedOutput() ([]byte, error) {
	content, err := sc.cmd.CombinedOutput()
	nerr := sc.cleanup()
	if err != nil {
		return nil, err
	}
	if nerr != nil {
		log.Warnf("Error waiting for command cleanup: %v", nerr)
	}
	return content, err
}

// Start starts the specified command but does not wait for it to complete.
//
// The Wait method will return the exit code and release associated resources
// once the command exits.
func (sc *SSHCommand) Start() error {
	return sc.cmd.Start()
}

// Display ...
func (sc *SSHCommand) Display() string {
	return strings.Join(sc.cmd.Args[:], " ")
}

// Run starts the specified command and waits for it to complete.
//
// The returned error is nil if the command runs, has no problems
// copying stdin, stdout, and stderr, and exits with a zero exit
// status.
//
// If the command starts but does not complete successfully, the error is of
// type *ExitError. Other error types may be returned for other situations.
func (sc *SSHCommand) Run() (int, string, string, error) {
	// Set up the outputs (std and err)
	stdOut, err := sc.StdoutPipe()
	if err != nil {
		return 0, "", "", err
	}
	stderr, err := sc.StderrPipe()
	if err != nil {
		return 0, "", "", err
	}

	// Launch the command and wait for its execution
	if err = sc.Start(); err != nil {
		return 0, "", "", err
	}

	msgOut, err := ioutil.ReadAll(stdOut)
	if err != nil {
		return 0, "", "", err
	}

	msgErr, err := ioutil.ReadAll(stderr)
	if err != nil {
		return 0, "", "", err
	}

	err = sc.Wait()
	if err != nil {
		log.Warnf("Error waiting for command: %v", err)
	}
	if err != nil {
		msgError, retCode, erro := ExtractRetCode(err)
		if erro != nil {
			return 0, "", "", err
		}
		return retCode, string(msgOut[:]), fmt.Sprint(string(msgErr[:]), msgError), nil
	}

	return 0, string(msgOut[:]), string(msgErr[:]), nil
}

// RunWithTimeout ...
func (sc *SSHCommand) RunWithTimeout(timeout time.Duration) (int, string, string, error) {
	log.Debugf("Running command [%s] with timeout of %s", sc.Display(), timeout)

	// Set up the outputs (std and err)
	stdOut, err := sc.StdoutPipe()
	if err != nil {
		return 0, "", "", err
	}
	stderr, err := sc.StderrPipe()
	if err != nil {
		return 0, "", "", err
	}

	// Launch the command and wait for its execution
	if err = sc.Start(); err != nil {
		return 0, "", "", err
	}

	doneCh := make(chan bool)

	var msgOut []byte
	var msgErr []byte

	go func() {
		defer close(doneCh)

		clean := true
		var closeErr error

		msgOut, closeErr = ioutil.ReadAll(stdOut)
		if closeErr != nil {
			log.Debugf("Error recovering standard output of command: %v", closeErr)
			clean = false
		}

		msgErr, closeErr = ioutil.ReadAll(stderr)
		if closeErr != nil {
			log.Debugf("Error recovering standard error of command: %v", closeErr)
			clean = false
		}

		err = sc.Wait()
		if err != nil {
			log.Debugf("Error waiting for command: %v", err)
			clean = false
		}

		doneCh <- clean
	}()

	select {
	case issues := <-doneCh:
		if err != nil {
			msgError, retCode, erro := ExtractRetCode(err)
			if erro != nil {
				return 0, "", "", err
			}
			return retCode, string(msgOut[:]), fmt.Sprint(string(msgErr[:]), msgError), nil
		}
		if !issues {
			log.Warnf("There have been issues running this command, please check daemon logs")
		}
	case <-time.After(timeout):
		errMsg := fmt.Sprintf("Timeout of (%s) waiting for the command to end", timeout)
		log.Warnf(errMsg)
		return 0, "", "", fmt.Errorf(errMsg)
	}

	return 0, string(msgOut[:]), string(msgErr[:]), nil
}

func (sc *SSHCommand) cleanup() error {
	err1 := sc.closeTunneling()
	err2 := utils.LazyRemove(sc.keyFile.Name())
	if err1 != nil {
		log.Errorf("closeTunneling() failed: %s\n", reflect.TypeOf(err1).String())
		return fmt.Errorf("Unable to close SSH tunnels: %s", err1.Error())
	}
	if err2 != nil {
		return fmt.Errorf("Unable to close SSH tunnels: %s", err2.Error())
	}
	return nil
}

func recCreateTunnels(ssh *SSHConfig, tunnels *[]*SSHTunnel) (*SSHTunnel, error) {
	if ssh != nil {
		tunnel, err := recCreateTunnels(ssh.GatewayConfig, tunnels)
		if err != nil {
			return nil, err
		}
		cfg := ssh
		if tunnel != nil {
			gateway := *ssh.GatewayConfig
			gateway.Port = tunnel.port
			gateway.Host = "127.0.0.1"
			cfg.GatewayConfig = &gateway
		}
		if cfg.GatewayConfig != nil {
			tunnel, err = buildTunnel(cfg)
			if err != nil {
				return nil, err
			}
			*tunnels = append(*tunnels, tunnel)
			return tunnel, err
		}
	}
	return nil, nil
}

// CreateTunneling ...
func (ssh *SSHConfig) CreateTunneling() ([]*SSHTunnel, *SSHConfig, error) {
	var tunnels []*SSHTunnel
	tunnel, err := recCreateTunnels(ssh, &tunnels)
	if err != nil {
		if err != nil {
			return nil, nil, fmt.Errorf("Unable to create SSH Tunnels : %s", err.Error())
		}
	}
	sshConfig := *ssh
	if tunnel == nil {
		return nil, &sshConfig, nil
	}

	if ssh.GatewayConfig != nil {
		if err != nil {
			return nil, nil, fmt.Errorf("Unable to create SSH Tunnel : %s", err.Error())
		}
		sshConfig.Port = tunnel.port
		sshConfig.Host = "127.0.0.1"
	}
	return tunnels, &sshConfig, nil
}

func createSSHCmd(sshConfig *SSHConfig, cmdString string, withSudo bool) (string, *os.File, error) {
	f, err := CreateTempFileFromString(sshConfig.PrivateKey, 0400)
	if err != nil {
		return "", nil, fmt.Errorf("Unable to create temporary key file: %s", err.Error())
	}

	options := sshOptions + " -oLogLevel=error"

	sshCmdString := fmt.Sprintf("ssh -i %s %s -p %d %s@%s",
		f.Name(),
		options,
		sshConfig.Port,
		sshConfig.User,
		sshConfig.Host,
	)

	sudoOpt := ""
	if withSudo {
		// tty option is required for some command like ls
		sudoOpt = " -t sudo"
	}

	if cmdString != "" {
		sshCmdString = sshCmdString + fmt.Sprintf("%s bash <<'ENDSSH'\n%s\nENDSSH", sudoOpt, cmdString)
	}
	return sshCmdString, f, nil

}

// Command returns the cmd struct to execute cmdString remotely
func (ssh *SSHConfig) Command(cmdString string) (*SSHCommand, error) {
	return ssh.command(cmdString, false)
}

// SudoCommand returns the cmd struct to execute cmdString remotely. Command is executed with sudo
func (ssh *SSHConfig) SudoCommand(cmdString string) (*SSHCommand, error) {
	return ssh.command(cmdString, true)
}

func (ssh *SSHConfig) command(cmdString string, withSudo bool) (*SSHCommand, error) {
	tunnels, sshConfig, err := ssh.CreateTunneling()
	if err != nil {
		return nil, fmt.Errorf("Unable to create command : %s", err.Error())
	}
	sshCmdString, keyFile, err := createSSHCmd(sshConfig, cmdString, withSudo)
	if err != nil {
		return nil, fmt.Errorf("Unable to create command : %s", err.Error())
	}
	cmd := exec.Command("bash", "-c", sshCmdString)
	sshCommand := SSHCommand{
		cmd:     cmd,
		tunnels: tunnels,
		keyFile: keyFile,
	}
	return &sshCommand, nil
}

// WaitServerReady waits until the SSH server is ready
// the 'timeout' parameter is in minutes
func (ssh *SSHConfig) WaitServerReady(phase string, timeout time.Duration) (string, error) {
	if phase == "" {
		panic("phase can't be empty string")
	}
	if ssh.Host == "" {
		panic("SSHConfig.Host is empty!")
	}
	log.Debugf("Waiting for remote SSH, timeout of %d minutes", int(timeout.Minutes()))

	if phase == "ready" {
		phase = "phase2"
	}

	var (
		retcode        int
		stdout, stderr string
	)
	err := retry.WhileUnsuccessfulDelay5Seconds(
		func() error {
			cmd, err := ssh.Command(fmt.Sprintf("sudo cat /opt/safescale/var/state/user_data.%s.done", phase))
			if err != nil {
				return err
			}

			// retcode, stdout, stderr, err := cmd.Run() // FIXME It CAN lock
			retcode, stdout, stderr, err = cmd.RunWithTimeout(timeout)
			if err != nil {
				return err
			}
			if retcode != 0 {
				if retcode == 255 {
					log.Debugf("Remote SSH not ready: error code: 255; Output [%s]; Error [%s]", stdout, stderr)
					return fmt.Errorf("remote SSH not ready: error code: 255")
				}
				log.Debugf("Remote SSH NOT ready: error code: %d; Output [%s]; Error [%s]", retcode, stdout, stderr)
				return fmt.Errorf("remote SSH not ready: error code: %s", stderr)
			}
			log.Debugf("Remote SSH ready: command finished with result: [%s]", stdout)
			return nil
		},
		timeout,
	)
	if err == nil {
		return stdout, nil
	}

	originalErr := err
	logCmd, err := ssh.Command(fmt.Sprintf("sudo cat /opt/safescale/var/log/user_data.%s.log", phase))
	if err != nil {
		return "", err
	}

	// retcode, stdout, stderr, logErr := logCmd.Run() // FIXME It CAN lock
	retcode, stdout, stderr, logErr := logCmd.RunWithTimeout(timeout)
	if logErr == nil {
		if retcode == 0 {
			return "", fmt.Errorf("server '%s' is not ready yet: %s - log content of file user_data.%s.log: %s", ssh.Host, originalErr.Error(), phase, stdout)
		}
		if len(stdout) > 0 {
			log.Error(fmt.Errorf("captured output: %s", stdout))
		}
		return "", fmt.Errorf("server '%s' is not ready yet: %s - error reading user_data.%s.log: %s", ssh.Host, originalErr.Error(), phase, stderr)
	}

	return "", fmt.Errorf("server '%s' is not ready yet: %s", ssh.Host, originalErr.Error())
}

// Copy copies a file/directory from/to local to/from remote
func (ssh *SSHConfig) Copy(remotePath, localPath string, isUpload bool) (int, string, string, error) {
	tunnels, sshConfig, err := ssh.CreateTunneling()
	if err != nil {
		return 0, "", "", fmt.Errorf("unable to create tunnels : %s", err.Error())
	}

	identityfile, err := CreateTempFileFromString(sshConfig.PrivateKey, 0400)
	if err != nil {
		return 0, "", "", fmt.Errorf("unable to create temporary key file: %s", err.Error())
	}
	log.Debugf("SSH Identity file: %s", identityfile.Name())

	cmdTemplate, err := template.New("Command").Parse("scp -i {{.IdentityFile}} -P {{.Port}} {{.Options}} {{if .IsUpload}}'{{.LocalPath}}' {{.User}}@{{.Host}}:'{{.RemotePath}}'{{else}}{{.User}}@{{.Host}}:'{{.RemotePath}}' '{{.LocalPath}}'{{end}}")
	if err != nil {
		return 0, "", "", fmt.Errorf("error parsing command template: %s", err.Error())
	}

	options := sshOptions + " -oLogLevel=error"
	var copyCommand bytes.Buffer
	if err := cmdTemplate.Execute(&copyCommand, struct {
		IdentityFile string
		Port         int
		Options      string
		User         string
		Host         string
		RemotePath   string
		LocalPath    string
		IsUpload     bool
	}{
		IdentityFile: identityfile.Name(),
		Port:         sshConfig.Port,
		Options:      options,
		User:         sshConfig.User,
		Host:         sshConfig.Host,
		RemotePath:   remotePath,
		LocalPath:    localPath,
		IsUpload:     isUpload,
	}); err != nil {
		return 0, "", "", fmt.Errorf("error executing template: %s", err.Error())
	}

	sshCmdString := copyCommand.String()
	log.Debugf("cmd: %s", sshCmdString)
	cmd := exec.Command("bash", "-c", sshCmdString)
	sshCommand := SSHCommand{
		cmd:     cmd,
		tunnels: tunnels,
		keyFile: identityfile,
	}

	return sshCommand.Run() // FIXME It CAN lock
}

// Exec executes the cmd using ssh
func (ssh *SSHConfig) Exec(cmdString string) error {
	tunnels, sshConfig, err := ssh.CreateTunneling()
	if err != nil {
		for _, t := range tunnels {
			nerr := t.Close()
			if nerr != nil {
				log.Warnf("Error closing ssh tunnel: %v", nerr)
			}
		}
		return fmt.Errorf("unable to create command : %s", err.Error())
	}
	sshCmdString, keyFile, err := createSSHCmd(sshConfig, cmdString, false)
	if err != nil {
		for _, t := range tunnels {
			nerr := t.Close()
			if nerr != nil {
				log.Warnf("Error closing ssh tunnel: %v", nerr)
			}
		}
		if keyFile != nil {
			nerr := utils.LazyRemove(keyFile.Name())
			if nerr != nil {
				log.Warnf("Error removing file %v", nerr)
			}
		}
		return fmt.Errorf("unable to create command : %s", err.Error())
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		for _, t := range tunnels {
			nerr := t.Close()
			if nerr != nil {
				log.Warnf("Error closing ssh tunnel: %v", nerr)
			}
		}
		if keyFile != nil {
			nerr := utils.LazyRemove(keyFile.Name())
			if nerr != nil {
				log.Warnf("Error removing file %v", nerr)
			}
		}
		return fmt.Errorf("unable to create command : %s", err.Error())
	}
	var args []string
	if cmdString == "" {
		args = []string{sshCmdString}
	} else {
		args = []string{"-c", sshCmdString}
	}
	err = syscall.Exec(bash, args, nil)
	nerr := utils.LazyRemove(keyFile.Name())
	if nerr != nil {
		log.Warnf("Error removing (lazy) file %v", nerr)
	}
	return err
}

// Enter Enter to interactive shell
func (ssh *SSHConfig) Enter() error {
	tunnels, sshConfig, err := ssh.CreateTunneling()
	if err != nil {
		for _, t := range tunnels {
			nerr := t.Close()
			if nerr != nil {
				log.Warnf("Error closing ssh tunnel: %v", nerr)
			}
		}
		return fmt.Errorf("unable to create command : %s", err.Error())
	}

	sshCmdString, keyFile, err := createSSHCmd(sshConfig, "", false)
	if err != nil {
		for _, t := range tunnels {
			nerr := t.Close()
			if nerr != nil {
				log.Warnf("Error closing ssh tunnel: %v", nerr)
			}
		}
		if keyFile != nil {
			nerr := utils.LazyRemove(keyFile.Name())
			if nerr != nil {
				log.Warnf("Error removing file %v", nerr)
			}
		}
		return fmt.Errorf("unable to create command : %s", err.Error())
	}

	bash, err := exec.LookPath("bash")
	if err != nil {
		for _, t := range tunnels {
			nerr := t.Close()
			if nerr != nil {
				log.Warnf("Error closing ssh tunnel: %v", nerr)
			}
		}
		if keyFile != nil {
			nerr := utils.LazyRemove(keyFile.Name())
			if nerr != nil {
				log.Warnf("Error removing file %v", nerr)
			}
		}
		return fmt.Errorf("unable to create command : %s", err.Error())
	}

	proc := exec.Command(bash, "-c", sshCmdString)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	err = proc.Run()
	nerr := utils.LazyRemove(keyFile.Name())
	if nerr != nil {
		log.Warnf("Error removing file %v", nerr)
	}
	return err
}

// CommandContext is like Command but includes a context.
//
// The provided context is used to kill the process (by calling
// os.Process.Kill) if the context becomes done before the command
// completes on its own.
func (ssh *SSHConfig) CommandContext(ctx context.Context, cmdString string) (*SSHCommand, error) {
	tunnels, sshConfig, err := ssh.CreateTunneling()
	if err != nil {
		return nil, fmt.Errorf("unable to create command : %s", err.Error())
	}
	sshCmdString, keyFile, err := createSSHCmd(sshConfig, cmdString, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create command : %s", err.Error())
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", sshCmdString)
	sshCommand := SSHCommand{
		cmd:     cmd,
		tunnels: tunnels,
		keyFile: keyFile,
	}
	return &sshCommand, nil
}

// CreateKeyPair creates a key pair
func CreateKeyPair() (publicKeyBytes []byte, privateKeyBytes []byte, err error) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	publicKey := privateKey.PublicKey
	pub, err := ssh.NewPublicKey(&publicKey)
	if err != nil {
		return nil, nil, err
	}

	publicKeyBytes = ssh.MarshalAuthorizedKey(pub)

	priBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyBytes = pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: priBytes,
		},
	)
	return publicKeyBytes, privateKeyBytes, nil
}