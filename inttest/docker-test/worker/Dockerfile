FROM ubuntu:latest
# RUN apt-get update 
# ADD ../mke /usr/local/bin
ADD ./entrypoint.sh /usr/local/bin/entrypoint.sh
# CMD [“echo”,”'Image created'”] 
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
EXPOSE 6443

