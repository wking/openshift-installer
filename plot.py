#!/usr/bin/python

import datetime
import json

import matplotlib.dates
import matplotlib.pyplot


with open('builds.json') as f:
    builds = json.load(f)


xs = []
ys = []
for start, build in builds.items():
    #start = datetime.datetime.fromisoformat(start)  # Python 3.7+
    start = datetime.datetime.strptime(start, '%Y-%m-%dT%H:%M:%S')
    xs.append(start)
    ys.append(build['duration'] / 60.)

figure = matplotlib.pyplot.figure()
axes = figure.add_subplot(1, 1, 1)
axes.set_title('duration of successful e2e-aws runs')
axes.set_ylabel('duration (minutes)')
axes.plot(xs, ys, '.')

axes.annotate(
    '#151 outage',
    xy=(
        datetime.datetime(2018, 8, 20),
        35,
    ),
    xytext=(
        datetime.datetime(2018, 8, 20),
        65,
    ),
    arrowprops=dict(width=1, facecolor='black', shrink=0.05),
)

axes.annotate(
    '#415 outage',
    xy=(
        datetime.datetime(2018, 10, 4),
        25,
    ),
    xytext=(
        datetime.datetime(2018, 10, 4),
        18,
    ),
    arrowprops=dict(width=1, facecolor='black', shrink=0.05),
)

figure.autofmt_xdate()
figure.savefig('builds.png')
