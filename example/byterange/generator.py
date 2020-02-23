# !/usr/bin/python3

def main():
    """
    在一个 target.txt 中放入数字 1-10000，每行一个
    """
    filename = 'target.txt'
    print('start')
    with open(filename, 'wt') as textfile:
        for i in range(0, 10000):
            print(str(i), file=textfile)
    print('down')


if __name__ == "__main__":
    main()
