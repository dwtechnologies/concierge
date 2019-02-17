import os
import boto3
import json
import json
import time
import serial
import datetime
from AWSIoTPythonSDK.MQTTLib import AWSIoTMQTTClient
from botocore.client import Config
from requests import Request, Session
from threading import Timer

topic_name = os.environ['IOT_TOPIC']
iot_endpoint = os.environ['IOT_ENDPOINT']
device_name = os.environ['DEVICE_NAME']
serial_device = os.environ['DeepLensDeviceReadAndWrite']
open_delay_seconds = os.environ['OPEN_DELAY_SECONDS']

# certificates
ca = "/etc/ssl/certs/ca-certificates.crt"
private = "/etc/greengrass-certs/cloud.pem.key"
cert = "/etc/greengrass-certs/cloud.pem.crt"

##https://www.amazon.co.uk/gp/product/B075F6J6WL/ref=ppx_yo_dt_b_asin_title_o02__o00_s01?ie=UTF8&psc=1 device
if serial_device is None:
    serial_device = "/dev/ttyUSB0"

last_update = datetime.datetime.now()
if serial_device is None:
    open_delay_seconds = 5


def is_online():
    print "ready to consume from topic '%s'" % topic_name

def custom_callback(client, userdata, message):
    try:
        global last_update

        print "received message from topic: %s" % message.topic
        payload = json.loads(message.payload)

        if 'command' not in payload:
            print "no command field in payload"
            return

        if 'username' not in payload:
            print "no username field in payload"
            return

        elapsed = datetime.datetime.now() - last_update
        if (elapsed.seconds < open_delay_seconds):
            print "received an open command, too soon"
            return

        if (payload['command'] == 'open'):
            print "opening door to %s" % payload['username']

            # lctech-inc.com usb relay
            #https://www.amazon.co.uk/gp/product/B075F6J6WL/ref=ppx_yo_dt_b_asin_title_o02__o00_s01?ie=UTF8&psc=1
            ser = serial.Serial(serial_device, 9600, timeout=1)
            # To close relay (ON)
            code = 'A00101A2'
            ser.write(code.decode('HEX'))

            # reset last_update time
            last_update = datetime.datetime.now()

            time.sleep(1)

            print "closing door"
            code = 'A00100A1'
            ser.write(code.decode('HEX'))
            ser.close()


        else:
            print "unknown command: %s" % payload['command']

    except Exception as e:
        print "crap, something failed: %s" % str(e)



def greengrass_infinite_infer_run():
    try:
        # certificate based connection
        myMQTTClient = AWSIoTMQTTClient(device_name)
        myMQTTClient.configureEndpoint(iot_endpoint, 8883)
        myMQTTClient.configureCredentials(ca, private, cert)

        myMQTTClient.configureAutoReconnectBackoffTime(1, 32, 20)
        myMQTTClient.configureOfflinePublishQueueing(-1)  # Infinite offline Publish queueing
        myMQTTClient.configureDrainingFrequency(2)  # Draining: 2 Hz
        myMQTTClient.configureConnectDisconnectTimeout(10)  # 10 sec
        myMQTTClient.configureMQTTOperationTimeout(5) # 5 sec

        print("connect to MQTT topic...")
        myMQTTClient.onOnline=is_online
        myMQTTClient.connect(keepAliveIntervalSecond=60)
        myMQTTClient.publish(topic_name, '{"message": "Connected!"}', 0)

        print('Subscribe')
        myMQTTClient.subscribe(topic_name, 1, custom_callback)
        time.sleep(2)
        loopCount = 0

        # always listen
        doInfer = True
        while doInfer:
            loopCount += 1
            time.sleep(1)

    except Exception as e:
        print "crap, something failed: %s" % str(e)

    # for resilience: asynchronously schedule this function to be run again in 10 seconds
    Timer(10, greengrass_infinite_infer_run).start()


# Execute the function above
greengrass_infinite_infer_run()


# This is a dummy handler and will not be invoked
# Instead the code above will be executed in an infinite loop for our example
def function_handler(event, context):
    return

