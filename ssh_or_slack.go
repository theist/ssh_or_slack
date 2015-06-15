package main

// Basic I/O
import "fmt"
import "io/ioutil"
import "bytes"
import "net/http"
// Time manip (sleep, constants)
import "time"
// A ini style reader
import "code.google.com/p/gcfg"
// A comand line parser
import "gopkg.in/alecthomas/kingpin.v2"
// official SSH package
import "golang.org/x/crypto/ssh"


// STRUCT for the INI file (substructs are sections, in the ini file vars are downcased and underscores translated to "-")
type Config struct {
  General struct {
    State_File string
    Sleep int64
  }
  Slack struct {
    Channel string
    Slack_Url string
    Ok_Message string
    Ko_Message string
  }
  Ssh struct {
    Ssh_Key_File string
    Host_Ip string
    Host_Port string
    User string
    Command string
  }
}

var (
  // Command line parser config
  debug   = kingpin.Flag("debug", "Enable debug mode.").Bool()
  config_file  = kingpin.Arg("config", "Configuration file.").Default(".do_ssh_to_host_or_tell_slack.rc").String()
  // state
  last_state = "KO"
  new_state = "OK"
  cfg Config
)

func getKeyFile() (key ssh.Signer, err error){
  file := cfg.Ssh.Ssh_Key_File
  buf, err := ioutil.ReadFile(file)
  if err != nil {
    return
  }
  key, err = ssh.ParsePrivateKey(buf)
  if err != nil {
    return
  }
  return
}


func main() {
  // Comand Line Config
  kingpin.Version("0.0.1")
  kingpin.Parse()
  if (*debug) {
    fmt.Printf("DBG: file: %s\n", *config_file)
  }
  // Ini file read
  err := gcfg.ReadFileInto(&cfg, *config_file)
  if (err != nil) {
    panic(err)
  }
  if (*debug) {
    fmt.Printf("DBG: Config %+v\n", cfg)
    fmt.Printf("DBG: Sleep %d %s\n", cfg.General.Sleep , cfg.General.State_File)
  }
  // SSH config
  key, err := getKeyFile();
  if err !=nil {
    panic(err)
  }
  ssh_cf := &ssh.ClientConfig {
    User: cfg.Ssh.User,
    Auth: []ssh.AuthMethod{
      ssh.PublicKeys(key),
    },
  }
  // main Loop
  for {
    //Sleep
    if (last_state != new_state){
      if (*debug) {
        fmt.Println ("DBG: State change from " + last_state + " to " + new_state)
      }
      last_state = new_state
      var payload []byte
      if new_state == "OK" {
        payload = []byte(`{"channel":"`+ cfg.Slack.Channel +`","attachments":[{"color":"good","text":"`+ cfg.Slack.Ok_Message + `"}]}`)

      } else {
        payload = []byte(`{"channel":"`+ cfg.Slack.Channel +`","attachments":[{"color":"danger","text":"`+ cfg.Slack.Ko_Message +`"}]}`)
      }
      fmt.Println(string(payload))
      req, err := http.NewRequest("POST", cfg.Slack.Slack_Url, bytes.NewBuffer(payload))
      req.Header.Set("Content-Type", "application/json")
      client := &http.Client{}
      resp, err := client.Do(req)
      defer resp.Body.Close()
      if (*debug) {
        fmt.Println("DBG: response Status:", resp.Status)
      }
      body, _ := ioutil.ReadAll(resp.Body)
      if (*debug) {
        fmt.Println("DBG: response Body:", string(body))
      }
      if err != nil {
        if (*debug) {
          fmt.Println ("Captured some error")
          fmt.Println (err)
        }
      }
    }

    time.Sleep (time.Duration(cfg.General.Sleep * int64(time.Second)))
    fmt.Printf(".")
    // Try to connect to SSH
    client, err := ssh.Dial("tcp",cfg.Ssh.Host_Ip + ":" + cfg.Ssh.Host_Port , ssh_cf)
    if err != nil {
      new_state = "KO"
      continue
    }
    session, err := client.NewSession()
    if err != nil {
      new_state = "KO"
      continue
    }
    defer session.Close()
    if err := session.Run(cfg.Ssh.Command); err != nil {
      if (*debug) {
        fmt.Println ("DBG: Executing ssh command")
      }
      new_state = "KO"
      continue
    }
    // Reach this All OK
    new_state = "OK"
  }
}
