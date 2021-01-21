FROM bluemeric/golang:1.12.6 
MAINTAINER Bluemeric <info@bluemeric.com> 

RUN mkdir /app/ 
WORKDIR /app/ 
 
###   Added artifacts into application working directory 
Add configurator /app/ 

CMD ["./configurator"]