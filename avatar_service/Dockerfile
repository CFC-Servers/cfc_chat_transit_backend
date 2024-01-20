FROM python:3.9.0

RUN pip install -U --upgrade pip

RUN mkdir /avatars
WORKDIR /avatar-service

COPY avatars.conf /etc/nginx/nginx.conf
COPY default.png /usr/share/nginx/html/avatars/default.png

COPY requirements.txt .
RUN pip install --upgrade -r ./requirements.txt

COPY service.py .

COPY entrypoint.sh /usr/bin/
ENTRYPOINT ["entrypoint.sh"]
