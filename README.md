Concierge (POC)
-------
Deeplens application that will identify a person with in its range, when it finds it will send it to a S3 bucket to be analyzed by the aws recognition service, if it is a known person it will put a open command to a mqtt topic that will be consumed by the same deeplens again that will trigger a USB relay. The USB relay should be connected to a "open door button" so the person can open the door without a access tag
[![Here is a demo of concierge service in action at daniel wellington office](http://img.youtube.com/vi/nysLLK3DOeg/0.jpg)](http://www.youtube.com/watch?v=nysLLK3DOeg)
Pre-requierment
-------
- Docker on the machine you will deploy from
- Deeplens
- The usb relay supported is the following,  https://www.amazon.co.uk/HALJIA-Module-Control-Intelligent-control/dp/B075F6J6WL/ref=sr_1_21?ie=UTF8&qid=1542143943&sr=8-21&keywords=usb+relay+5v
Make sure to check dmesg when you plugin the device, if it does not register as /dev/ttyUSB0 you need to update the variable DeepLensDeviceReadAndWrite with the right one

Setup
-----
Quite a few steps, needs cleanup, most of it can be automated.

- Fix 'parameters.conf' file to fit your environment
- Fix AWSDeepLensGreengrassGroupRole role with the S3 bucket permissions you defined in parameters.config
- Create a Rekognition collection (in the same region as in params.conf) using [aws cli](https://docs.aws.amazon.com/cli/latest/reference/rekognition/create-collection.html)

- Deploy the Lambda functions with make (verify env variables in the Makefile)

- Go into the deeplens console, and create a project, choose "Use a project template" select the "Object detection" model
- Remove the `deeplens-object-detection` function, and add a function for `find_person` and `trigger_open`
- If its empty when you start you can choose to add the deeplens-object-detection model and add the `find_person` and `trigger_open` function.
- Deploy the application to your deeplens

Troubleshooting
-----
If you run into any problems during the deployment to the deeplens, find the device in the greengras console and look at the deployment error you get
If it fails the first time you will the error "An error occurred (ValidationError) when calling the CreateChangeSet operation: Stack:arn:aws:cloudformation:us-east-1:494090316628:stack/concierge/6bab71c0-3357-11e9-892f-12b323895910 is in ROLLBACK_COMPLETE state and can not be updated.
", you need to manually delete the stack to rerun
Tweaking
-----
There is a hack due to there is no state on the deeplens / aws side, for every detection (every frame it processes) it will fire away a open command. To not let the trigger_open get 20 open commands at the same time, there is a deplay / ratelimit in the code to wait for X amount of seconds after the first open command. Default is 5 and it is defined in the file trigger_open.py as the variable open_delay_seconds 

Todo
-----
-Include nanpy code that worked with a arduino+relay over usb in a early stage before changed device to a usb relay
-Use the person with the highest area, it looks like it gets confused with multiple persons
-Trigger another lambda function that will confirm a second auth to the person to prevent any abuse by using a mask with a printed face on
...

Inspired by
-----
 [AWS Deeplens Hackaton](https://devpost.com/software/doorman-a1oh0e)

