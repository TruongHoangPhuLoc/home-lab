install image

set interfaces ethernet eth0 address 172.23.1.210/24

set protocols bgp system-as 64500

set protocols bgp neighbor 172.16.1.201 remote-as 64500

set protocols bgp neighbor 172.16.1.202 remote-as 64500

set protocols bgp neighbor 172.16.1.203 remote-as 64500

set protocols bgp neighbor 172.16.1.204 remote-as 64500
