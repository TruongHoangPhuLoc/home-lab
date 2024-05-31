FROM python:3.9.17-slim-buster
WORKDIR /app
COPY ./requirements.txt /app
RUN pip3 install -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["python3", "-u", "script.py"]