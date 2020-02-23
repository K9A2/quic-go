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

link_loss = 0.1

class SimpleTopology(Topo):
    def __init__(self):
        Topo.__init__(self)

        h1 = self.addHost('h1')
        h2 = self.addHost('h2')
        h3 = self.addHost('h3')
        h4 = self.addHost('h4')
        h5 = self.addHost('h5')
        s1 = self.addSwitch('s1')

        self.addLink(h1, s1, bw=100, delay='12.5ms', loss=link_loss)
        self.addLink(h2, s1, bw=100, delay='25ms', loss=link_loss)
        self.addLink(h3, s1, bw=100, delay='50ms', loss=link_loss)
        self.addLink(h4, s1, bw=100, delay='100ms', loss=link_loss)
        self.addLink(s1, h5)


def run():
    net = Mininet(topo=SimpleTopology(), host=CPULimitedHost, link=TCLink)
    net.start()
    CLI(net)
    net.stop()


if __name__ == '__main__':
    os.system('mn -c')
    setLogLevel('info')
    run()
