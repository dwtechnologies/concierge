# Concierge

A AWS Deeplens application that will identify a person with in its range, when it finds a person will send the picture to a S3 bucket to be analyzed by the aws recognition service. If it is a known person it will put a open command to a topic that will be consumed by the same deeplens again that will trigger a USB relay. The USB relay should be connected to a "open door button" so the person can open the door without a access tag

It is a fork of the Doorman community project.

Here is a demo, normally it takes 2-4s for the whole flow but in this case we where lucky with the speed of the setup.
[![Here is a demo of concierge service in action at Daniel Wellington office](http://img.youtube.com/vi/nysLLK3DOeg/0.jpg)](http://www.youtube.com/watch?v=nysLLK3DOeg)

Presentations
---
- [AWS Meetup - Spot instances and ECS, Re:invent recap and Deeplens @DW 2018-12](https://www.meetup.com/aws-stockholm/events/255772998/)
[![Video recording](http://img.youtube.com/vi/b6p2WG4a9A0/0.jpg)](https://www.youtube.com/watch?v=b6p2WG4a9A0)
- [AWS Community Day Copenhagen 2019-02](https://awscommunitynordics.org/communityday/)
[![Presentation material](https://image.slidesharecdn.com/awscommunityday2019q1-190219141955/95/aws-community-day-copenhagen-2019-bring-ml-awareness-with-deeplens-to-the-business-by-accessing-the-office-with-the-face-1-638.jpg)](https://www.slideshare.net/danielwellingtontechnology/aws-community-day-copenhagen-2019-bring-ml-awareness-with-deeplens-to-the-business-by-accessing-the-office-with-the-face)

Prerequisite
---
- Docker on the machine you will deploy from (tested on a Mac and Ubuntu 18.04
- [AWS Deeplens](https://aws.amazon.com/deeplens/)
- The USB relay supported is the following (for a 5V button),  https://www.amazon.co.uk/HALJIA-Module-Control-Intelligent-control/dp/B075F6J6WL/ref=sr_1_21?ie=UTF8&qid=1542143943&sr=8-21&keywords=usb+relay+5v
- Make sure to check dmesg when you plugin the device, if it does not register as /dev/ttyUSB0 you need to update the variable DeepLensDeviceReadAndWrite, same goes for variable serial_device inside trigger_open.py

Setup
---
Quite a few steps, needs cleanup, most of it can be automated.

- Fix 'parameters.conf' file to fit your environment
- You can find the IoTEndpoint value by running aws --region {region} iot describe-endpoint or the AWS IOT console (make sure you are in the right region) and settings. You will find the value as "Endpoint",  direct url https://console.aws.amazon.com/iot/home?region=<YOUR-REGION>#/settings
- Create a new policy and then attach it to the AWSDeepLensGreengrassGroupRole role, make sure to update the BucketName-from-parameters.conf from bellow to the actual value from parameters.conf
```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:PutObjectAcl"
            ],
            "Resource": [
                "arn:aws:s3:::BucketName-from-parameters.conf/*",
                "arn:aws:s3:::BucketName-from-parameters.conf"
            ]
        }
    ]
}
```
- Create a Rekognition collection (in the same region as in params.conf) using [aws cli](https://docs.aws.amazon.com/cli/latest/reference/rekognition/create-collection.html)

- Deploy the Lambda functions with make (verify env variables in the Makefile)

- Go into the deeplens console, and create a project, choose "Use a project template" select the "Object detection" model
- Remove the `deeplens-object-detection` function, and add a function for `find_person` and `trigger_open`
- If its empty when you start you can choose to add the deeplens-object-detection model and add the `find_person` and `trigger_open` function.
- Deploy the application to your deeplens

Troubleshooting
---
- If you run into any problems during the deployment to the deeplens, find the device in the Greengras console and look at the deployment error you get.
- If it fails the first time you will the error "An error occurred (ValidationError) when calling the CreateChangeSet operation: Stack:arn:aws:cloudformation:us-east-1:494090316628:stack/concierge/6bab71c0-3357-11e9-892f-12b323895910 is in ROLLBACK_COMPLETE state and can not be updated.", you need to manually delete the stack to rerun.
- If your door opening commands does not triggers and gets consumed by the device, it is most likely due to the name of the device does not match, check the lambda logs for concierge-trigger-open (from the deeplens console). The logs will show something similar to [INFO]-mqtt_core.py:88,Client id: deeplens_*, make sure that this matches with the value DeviceNeme from parameters.conf 

Testing
---
You can test the "door open" flow by going into the greengras console, testing. In the botton part (Publish), put in the value of your topic (found as IoTTopic in the parameters.conf) and the data bellow
```
{
  "username": "Lezgin.Bakircioglu",
  "command": "open"
}
```

Audit log
---
Any door openings is logged in cloudwatch logs, you can find them by browsing to the device and then lambda logs.
You will find the log concierge-trigger-open, example bellow
```
[INFO]-trigger_open.py:44,received message from topic: doorman
[INFO]-trigger_open.py:61,opening door to Lezgin.Bakircioglu
[INFO]-trigger_open.py:75,closing door
```

Tweaking
---
There is a hack due to there is no state on the deeplens / aws side, for every detection (every frame it processes) it will fire away a open command. To not let the trigger_open get 20 open commands at the same time, there is a deplay / ratelimit in the code to wait for X amount of seconds after the first open command. Default is 5 and it is defined in the file trigger_open.py as the variable open_delay_seconds 

Todo
---
- Include nanpy code that worked with a arduino+relay over usb in a early stage before changed device to a usb relay
- Use the person with the highest area, it looks like it gets confused with multiple persons
- Trigger another lambda function that will confirm a second auth to the person to prevent any abuse by using a mask with a printed face on
- Structure the README file and identify the missing parts that a new user would bump into
...

Inspired by
---
[AWS Deeplens Hackaton](https://devpost.com/software/doorman-a1oh0e)

