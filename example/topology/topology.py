# /usr/bin/python
# coding:utf-8

import os
import time

from mininet.cli import CLI
from mininet.link import TCLink
from mininet.log import setLogLevel
from mininet.net import Mininet
from mininet.node import CPULimitedHost
from mininet.topo import Topo

cable = {
	'bandwidth': 100,
	'delay': str(10 / 2) + 'ms',
	'loss': 0 / 2
}

wifi = {
	'bandwidth': 30,
	'delay': str(20 / 2) + 'ms',
	'loss': 0 / 2
}

lte = {
	'bandwidth': 5,
	'delay': str(60 / 2) + 'ms',
	'loss': 0 / 2
}

current_topo = wifi

class SimpleTopology(Topo):
    """
    h1 <-> s1 <- bottleneck -> s2 <-> h2
    """
    def __init__(self):
        Topo.__init__(self)

        sender = self.addHost('h1')
        receiver = self.addHost('h2')
        switch_1 = self.addSwitch('s1')
        switch_2 = self.addSwitch('s2')

        self.addLink(sender, switch_1)
        self.addLink(receiver, switch_2)

        self.addLink(switch_1, switch_2,
            bw=current_topo['bandwidth'], 
            delay=current_topo['delay'], 
            loss=current_topo['loss']
        )


def run():
    net = Mininet(topo=SimpleTopology(), host=CPULimitedHost, link=TCLink)
    net.start()
    CLI(net)
    net.stop()


if __name__ == '__main__':
    os.system('mn -c')
    setLogLevel('info')
    run()
