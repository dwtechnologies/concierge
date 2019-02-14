Doorman
-------
Deeplens application to notify when a user enters a room.


Setup
-----
Quite a few steps, needs cleanup, most of it can be automated.

- Create a Rekognition collection
- Fix 'parameters.conf' file to fit your environment
- Fix AWSDeepLensGreengrassGroupRole role with S3 permissions

- Deploy the Lambda functions with make (verify env variables in the Makefile)

- Go into the deeplens console, and create a project, select the "Object detection" model
- Remove the `deeplens-object-detection` function, and add a function for `find_person`
- Deploy the application to your deeplens

...

Inspired by
-----
 [AWS Deeplens Hackaton](https://devpost.com/software/doorman-a1oh0e)

