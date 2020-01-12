import json
import base64

DOAMIN_INDEX = 2
RESOURCE_INDEX = -1

def write(file_path, content, mode):
    with open(file_path, mode) as output_file:
        output_file.write(content)


def is_text_file(mime_type):
    text_file_types = [
        'text/html', 'text/plain', 'text/javascript', 'text/css', 'application/json',
    ]
    # binary_file_types = [
    #     'font/woff2', 'image/jpeg', 'image/webp', 'image/gif'
    # ]

    if any(mime_type in s for s in text_file_types):
        return True
    return False


def main():
    harfile_path = 'www.youtube.com.har'
    output_folder = './youtube/'
    harfile = open(harfile_path)
    harfile_json = json.loads(harfile.read())

    entries = harfile_json['log']['entries']
    numEntries = len(entries)
    print('numEntries: %d' % numEntries)

    domains = []
    index = 1
    for entry in entries:
        # parse fields from request
        request = entry['request']
        params = request['url'].split('/')

        method = request['method']
        resource_name = params[RESOURCE_INDEX]
        domain = params[DOAMIN_INDEX]
        decoded = False

        # parse fields from response
        response = entry['response']
        mime_type = response['content']['mimeType']
        content = response['content']['text']
        encoding = ''
        if 'encoding' in response['content']:
            encoding = response['content']['encoding']
        if encoding == 'base64':
            content = base64.b64decode(content)
            decoded = True

        if resource_name == '':
            resource_name = 'index.html'
        print('processing: %s' % resource_name)
        if is_text_file(mime_type):
            if decoded:
                content = content.decode('utf-8')
            write('./youtube/' + resource_name, content, 'w')
        else:
            write('./youtube/' + resource_name, content, 'wb')


if __name__ == "__main__":
    main()
