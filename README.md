# Smart Meter Bridge

Smart Meter Bridge allows the user to expose a serial connection to a DSMR smartmeter as a TCP/IP service. It's primary 
use case is to be used in conjunction with [Home Assistant](https://www.home-assistant.io/) in situations where the 
device running Home Assistant is not directly connected to the P1 port of a DSMR smart meter. There are several hardware
options to provide a solution in that case, but for people (like me) who already have a raspberry pi lying around, a 
software solution is better.

## Why not [ser2net](https://ser2net.sourceforge.net/)?
Ser2net offers a lot more features than are needed in this particular use-case. Smart Meter Bridge is written in GoLang 
which allows it to be compiled to a single executable file that can run without dependencies.

## Quick start
- Download the binary for your system.
- Create a file named `config.yml` in the folder containing the binary
- Add the following lines to `config.yml`:
```yaml
serial_port: "/path/to/your/serial/port"
dsmr_version: "DSMR VERION OF YOUR METER"
server:
  port: 9988
```
- Execute the binary
