FROM python:3.9.17-slim-buster
WORKDIR /app
COPY ./requirements.txt /app
RUN pip3 install -r requirements.txt
COPY . .
EXPOSE 5000
CMD ["flask", "run", "--host", "0.0.0.0"]
