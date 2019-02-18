
ENVIRONMENT        ?= prod
PROJECT            =  itops
STACK_NAME         =  concierge
ARTIFACTS_BUCKET   =  bucket-name-for-lambda-deployment
AWS_DEFAULT_REGION ?= us-east-1

sam_package = aws cloudformation package \
                --template-file sam.yaml \
                --output-template-file dist/sam.yaml \
                --s3-bucket $(ARTIFACTS_BUCKET)

sam_deploy = aws cloudformation deploy \
                --template-file dist/sam.yaml \
                --stack-name $(STACK_NAME) \
		--region $(AWS_DEFAULT_REGION) \
                --parameter-overrides \
                        $(shell cat parameters.conf) \
                --capabilities CAPABILITY_IAM \
                --no-fail-on-empty-changeset

deploy:
	@mkdir -p dist
	# golang
	cd source/guess; GOOS=linux go build -ldflags="-s -w" -o main && zip deployment.zip main
	cd source/unknown; GOOS=linux go build -ldflags="-s -w" -o main && zip deployment.zip main
	cd source/train; GOOS=linux go build -ldflags="-s -w" -o main && zip deployment.zip main
	# python
	cd source/find-person; mkdir dist \
		&& cp find_person.py dist/ \
		&& cd dist; zip deployment.zip *
	docker run -v ${PWD}/source/trigger-open:/app -w /app -it python:2.7-alpine sh -c "pip install -r requirements.txt -t ./dist; chmod -R 777 dist"
		cd source/trigger-open && cp trigger_open.py dist/ \
		&& cd dist/ && zip -r deployment.zip * && cp deployment.zip /tmp&& cp deployment.zip /tmp
	# sam
	$(call sam_package)
	$(call sam_deploy)
	@rm -rf source/*/main source/*/deployment.zip source/*/dist dist

clean:
	@rm -rf source/*/main source/*/deployment.zip source/*/dist dist


