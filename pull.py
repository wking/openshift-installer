#!/usr/bin/python3

import collections
import datetime
import json
import re
import urllib.error
import urllib.parse
import urllib.request


URI='https://gcsweb-ci.svc.ci.openshift.org/gcs/origin-ci-test/pr-logs/pull/openshift_installer/'
href_regexp = re.compile('href="(?P<href>[^"]*)')
builds = {}


try:
    with open('builds.json') as f:
        builds = json.load(f)
except FileNotFoundError:
    pass


def get(uri):
    with urllib.request.urlopen(uri) as response:
        content_bytes = response.read()
        charset = response.headers.get_content_charset()
    return content_bytes.decode(charset)


for pr in range(1, 10000):
    try:
        content = get(uri='{}{}/'.format(URI, pr))
    except urllib.error.HTTPError as error:
        print(error)
        continue
    e2e_aws = None
    for match in href_regexp.finditer(content):
        href = match.group('href')
        if href.endswith('e2e-aws/'):
            e2e_aws = href
            break
    if e2e_aws is None:
        continue
    print(e2e_aws)
    try:
        content = get(uri=urllib.parse.urljoin(URI, e2e_aws))
    except urllib.error.HTTPError as error:
        print(error)
        continue
    jobs = []
    for match in href_regexp.finditer(content):
        href = match.group('href')
        if href.startswith(e2e_aws):
            jobs.append(href)
    for job in jobs:
        job_uri = urllib.parse.urljoin(URI, job)
        try:
            finished = json.loads(get(uri=urllib.parse.urljoin(job_uri, 'finished.json')))
        except urllib.error.HTTPError as error:
            continue
        print(job_uri, finished)
        success = False
        if finished.get('passed', False):
            success = True
        elif finished.get('result') == 'SUCCESS':
            success = True
        if not success:
            continue
        started = json.loads(get(uri=urllib.parse.urljoin(job_uri, 'started.json')))
        start = datetime.datetime.fromtimestamp(started['timestamp'])
        duration = finished['timestamp'] - started['timestamp']
        builds[start.isoformat()] = {
            'uri': job_uri,
            'duration': duration,
            'pull-request': pr,
        }

    with open('builds.json', 'w') as f:
        json.dump(builds, f, sort_keys=True, indent=2)
        f.write('\n')
