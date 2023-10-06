from datetime import datetime, time, timedelta
import time as tm
import sys
import os
import json
import argparse
from zoneinfo import ZoneInfo

parser = argparse.ArgumentParser()
parser.add_argument('--save', action='store_true')
parser.add_argument('--single', action='store_true')
args = parser.parse_args()

utc = ZoneInfo('UTC')
tokyo = ZoneInfo('Asia/Tokyo')
now = datetime.now().astimezone(utc)

bossList = ["Wandering Death", "Avarice", "Ashava"]
timeList = ["5:53:30", "5:25:13", "5:53:29", "5:53:30", "5:25:13"]
repeatCountList = [3, 2, 3, 2]

currentWorldBossSpawnTime = datetime.strptime("05:14:33", '%H:%M:%S')
bossIndex = 0
timeIndex = 1
repeatCountIndex = 3
currentRepeatCount = 1

if os.path.isfile('boss.json'):
    with open('boss.json', 'r') as f:
        cfg = json.load(f)
    currentWorldBossSpawnTime = datetime.strptime(cfg['spawnTime'], '%H:%M:%S')
    bossIndex = int(cfg['bossIndex'])
    timeIndex = int(cfg['timeIndex'])
    repeatCountIndex = int(cfg['repeatCountIndex'])
    currentRepeatCount = int(cfg['currentRepeatCount'])

nextSpawnTime = currentWorldBossSpawnTime.replace(tzinfo=utc)
nextSpawnTime = datetime.combine(now.date(), nextSpawnTime.time()).replace(tzinfo=utc)

def out_of_bounds(t):
    # Define the ranges.
    range0_start = time(4, 30)
    range0_end = time(6, 30)
    range1_start = time(10, 30)
    range1_end = time(12, 30)
    range2_start = time(16, 30)
    range2_end = time(18, 30)
    range3_start = time(22, 30)
    range3_end = time(0, 30)

    # Check if the time is out of bounds.
    if not ((range1_start <= t <= range1_end) or (range2_start <= t <= range2_end) or (range3_start <= t <= range3_end) or (range0_start <= t <= range0_end)):
        return True

    return False

while True:
    if currentRepeatCount >= repeatCountList[repeatCountIndex]:
        bossIndex += 1
        if bossIndex >= len(bossList):
            bossIndex = 0
        repeatCountIndex += 1
        if repeatCountIndex >= len(repeatCountList):
            repeatCountIndex = 0
        currentRepeatCount = 0
    timeToAdd = timedelta(hours=int(timeList[timeIndex].split(':')[0]), minutes=int(timeList[timeIndex].split(':')[1]), seconds=int(timeList[timeIndex].split(':')[2]))
    nextSpawnTime += timeToAdd
    if out_of_bounds(nextSpawnTime.time()):
        nextSpawnTime += timedelta(hours=2)
    nextWorldBoss = bossList[bossIndex]

    # print("Next spawn time: " + str(nextSpawnTime.astimezone(tokyo)) + " " + nextWorldBoss)
    timeIndex += 1
    if timeIndex >= len(timeList):
        timeIndex = 0
    currentRepeatCount += 1
    
    if nextSpawnTime < now:
        cfg = {
            'spawnTime': nextSpawnTime.strftime('%H:%M:%S'),
            'bossIndex': bossIndex,
            'timeIndex': timeIndex,
            'repeatCountIndex': repeatCountIndex,
            'currentRepeatCount': currentRepeatCount
        }
        with open('boss.json', 'w') as f:
            json.dump(cfg, f)
        # don't output past spawn
        continue

    print(json.dumps({
        "time": int((int(datetime.timestamp(nextSpawnTime)) - int(datetime.timestamp(now))) / 60),
        "date": str(nextSpawnTime.astimezone(tokyo)),
        "name": nextWorldBoss
    }))

    if args.single:
        exit(0)
    tm.sleep(1)
    
