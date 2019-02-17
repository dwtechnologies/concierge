Doorman (POC)
-------
Deeplens application to notify when a user enters a room.

Hardware
-------
The usb relay supported is the following,  https://www.amazon.co.uk/HALJIA-Module-Control-Intelligent-control/dp/B075F6J6WL/ref=sr_1_21?ie=UTF8&qid=1542143943&sr=8-21&keywords=usb+relay+5v
Make sure to check dmesg when you plugin the device, if it does not register as /dev/ttyUSB0 you need to update the variable DeepLensDeviceReadAndWrite with the right one

Setup
-----
Quite a few steps, needs cleanup, most of it can be automated.

- Fix 'parameters.conf' file to fit your environment
- Fix AWSDeepLensGreengrassGroupRole role with the S3 bucket permissions you defined in parameters.config
- Create a Rekognition collection (in the same region as in params.conf) using [aws cli](https://docs.aws.amazon.com/cli/latest/reference/rekognition/create-collection.html)

- Deploy the Lambda functions with make (verify env variables in the Makefile)

- Go into the deeplens console, and create a project, select the "Object detection" model
- Remove the `deeplens-object-detection` function, and add a function for `find_person`
- Deploy the application to your deeplens

Troubleshooting
-----
If you run into any problems during the deployment to the deeplens, find the device in the greengras console and look at the deployment error you get

Tweaking
-----
There is a hack due to there is no state on the deeplens / aws side, for every detection (every frame it processes) it will fire away a open command. To not let the door get 20 door commands at the same time, there is a deplay / ratelimit in the code to wait for X amount of seconds after the first open command. Default is 5 and it is defined in the file trigger_open.py as the variable open_delay_seconds 

Todo
-----
-Include nanpy code that worked with a arduino+relay over usb in a early stage before changed device to a usb relay
...

Inspired by
-----
 [AWS Deeplens Hackaton](https://devpost.com/software/doorman-a1oh0e)

