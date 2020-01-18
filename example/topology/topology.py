# coding:utf-8

import os
import time

from mininet.cli import CLI
from mininet.link import TCLink
from mininet.log import setLogLevel
from mininet.net import Mininet
from mininet.node import CPULimitedHost
from mininet.topo import Topo

# rtt = [10, 50, 100, 200]
rtt = 20
# loss = [0.001, 0.01, 0.1, 1.0]
l = 0.0
bandwidth = 4

class SimpleTopology(Topo):
    def __init__(self):
        Topo.__init__(self)

        sender_name = 'h1'
        receiver_name = 'h2'

        switch_name = 's1'

        sender = self.addHost(sender_name)
        receiver = self.addHost(receiver_name)
        switch = self.addSwitch(switch_name)

        self.addLink(sender, switch, bw=bandwidth, delay=str(rtt / 4) + 'ms', loss=l)
        self.addLink(receiver, switch, bw=bandwidth, delay=str(rtt / 4) + 'ms', loss=l)


def run():
    net = Mininet(topo=SimpleTopology(), host=CPULimitedHost, link=TCLink)
    net.start()
    CLI(net)
    net.stop()


if __name__ == '__main__':
    os.system('mn -c')
    setLogLevel('info')
    run()
