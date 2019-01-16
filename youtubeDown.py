from datetime import datetime
import enum
import json
import os
import os.path
import requests
import string
import sys


class YT_Parameter(enum.IntEnum):
    AUDIO = 1
    VIDEO = 2
    BOTH = 3


def integer_to_bkbmb(integer: int) -> str:
    prefix = [
        "B", "KiB", "MiB", "GiB", "TiB"
    ]
    stored = float(integer)
    counter = 0

    while True:
        integer //= 1024
        if integer > 0:
            counter += 1
            if counter == 4:
                break
        else:
            break

    numerical_part = stored / (1024 ** counter)
    return "{:.2f}{:s}".format(numerical_part, prefix[counter])


def callback(dictionery: dict):
    id = dictionery["id"]
    downloaded = int(dictionery["downloaded"])
    total = int(dictionery["total"])
    speed = int(dictionery["avg_speed"])
    print(" " * 80, end="\r")
    print("{id} Downloading: "
          "{downloaded}/{total}"
          "{speed}/s)".format(
              id=id, downloaded=integer_to_bkbmb(downloaded),
              total=integer_to_bkbmb(total), speed=integer_to_bkbmb(speed)
          ), end="\r")


def valid_filename(string_to: str) -> str:
    valid_chars = "-_.() {}{}".format(string.ascii_letters, string.digits)
    return "".join(c for c in string_to if c in valid_chars)


class YT:
    def __init__(self, link_str: str, parameter: YT_Parameter):
        self.link = link_str
        self.parameter = parameter
        self.session = requests.Session()

    @staticmethod
    def parse_out(html: str) -> str:
        parsed = html
        parsed = parsed.replace("\\\\", "\\")
        parsed = parsed.replace("\\\"", "\"")
        parsed = parsed.replace("\\/", "/")
        parsed = parsed.replace("\\u0026", "&")

        return parsed

    @staticmethod
    def get_main_section(html: str) -> str:
        section_start = html.find("\"player_response\"")
        section_end = html.find(";ytplayer.load", section_start)

        main_section = html[section_start: section_end]
        return YT.parse_out(main_section)

    @staticmethod
    def get_jsons_string(parsed_str: str) -> str:
        parsed_main_section = YT.get_main_section(parsed_str)
        section_start = parsed_main_section.find("\"adaptiveFormats\"")
        section_start += len("\"adaptiveFormats\":")

        section_end = parsed_main_section.find("]", section_start)

        json_strings = parsed_main_section[section_start: section_end + 1]
        return json_strings

    @staticmethod
    def get_jsons(jsons_string: str) -> list:
        jsons_string = YT.get_jsons_string(jsons_string)
        return json.loads(jsons_string)

    @staticmethod
    def get_video_detail(html: str) -> dict:
        parsed_main_section = YT.get_main_section(html)
        section_start = parsed_main_section.find("\"videoDetails\"")
        section_start += len("\"videoDetails\":")

        section_end = parsed_main_section.find("},\"playerConfig\"",
                                               section_start)
        video_detail_str = parsed_main_section[section_start: section_end + 1]
        return json.loads(video_detail_str)

    @staticmethod
    def get_highest_bitrate(jsons: list, parameter: YT_Parameter,
                            width: int = 0) -> (int, int):
        audio_find = (parameter == YT_Parameter.AUDIO
                      or parameter == YT_Parameter.BOTH)
        video_find = (parameter == YT_Parameter.VIDEO
                      or parameter == YT_Parameter.BOTH)

        highest_bitrate_audio = -1
        highest_bitrate_video = -1
        index_audio = -1
        index_video = -1
        for index, json_dict in enumerate(jsons):
            if audio_find:
                if "audio/" in json_dict["mimeType"]:
                    if int(json_dict["bitrate"]) > highest_bitrate_audio:
                        highest_bitrate_audio = int(json_dict["bitrate"])
                        index_audio = index
            if video_find:
                if "video/" in json_dict["mimeType"]:
                    if int(json_dict["bitrate"]) > highest_bitrate_video and\
                            int(json_dict["width"]) > width:
                        highest_bitrate_video = int(json_dict["bitrate"])
                        index_video = index

        return index_audio, index_video

    @staticmethod
    def download_internal(detail: dict, json: dict,
                          called_function=None):
        id = detail["videoId"]
        just_name = "{name}-{id}".format(name=detail["title"],
                                         id=detail["videoId"])
        name = just_name
        url = json["url"]

        start = json["mimeType"].find("/")
        end = json["mimeType"].find(";")
        if start != -1 and end != -1:
            ext = json["mimeType"][start + 1: end]
            if "audio" in json["mimeType"]:
                name = name + "_audio." + ext
            else:
                name = name + "_video." + ext
        name = valid_filename(name).strip()

        r = requests.get(url, stream=True)
        total_size = int(r.headers["Content-length"])
        if os.path.isfile(name):
            if os.stat(name).st_size == total_size:
                print("{id} Already downloaded".format(id=id), end="")
                return name

        if os.path.isfile(just_name + ".mkv"):
            print("{id} Already downloaded".format(id=id), end="")
            return name
        downloaded = 0

        start_time = datetime.now()
        with open(name, "wb") as file:
            for chunk in r.iter_content(chunk_size=1024):
                if chunk:
                    file.write(chunk)
                    if called_function is not None:
                        downloaded += len(chunk)

                        now_time = datetime.now()
                        delta = (now_time - start_time).total_seconds()
                        estimated_time = round(total_size / downloaded * delta)
                        avg_speed = round(downloaded / delta)

                        dictionery = {
                            "id": id,
                            "downloaded": downloaded,
                            "total": total_size,
                            "estimated": estimated_time,
                            "avg_speed": avg_speed
                        }
                        called_function(dictionery)

        return name

    def download(self, called_function=None):
        response = self.session.get(self.link)
        jsons = YT.get_jsons(response.text)
        video_detail = YT.get_video_detail(response.text)

        audio_index, video_index = YT.get_highest_bitrate(
            jsons, self.parameter)

        audio_json = jsons[audio_index] if audio_index != -1 else None
        video_json = jsons[video_index] if video_index != -1 else None
        if video_json is not None:
            video_name =\
                YT.download_internal(video_detail, video_json, called_function)
            print("")

        if audio_json is not None:
            audio_name =\
                YT.download_internal(video_detail, audio_json, called_function)
            print("")

        if self.parameter == YT_Parameter.BOTH:
            if os.path.isfile("ffmpeg.exe"):
                if os.path.isfile(video_name) and os.path.isfile(audio_name):
                    id = video_detail["videoId"]
                    print("{id} Combining".format(id=id))
                    full_name =\
                        video_name[: video_name.rfind("_video")] + ".mkv"

                    os.system("ffmpeg -i \"{video_name}\" -i \"{audio_name}\""
                              " -c:v copy -c:a copy -y \"{full_name}\""
                              " >nul 2>&1".format(
                                  video_name=video_name, audio_name=audio_name,
                                  full_name=full_name
                              ))
                    os.remove(video_name)
                    os.remove(audio_name)
            else:
                print("ffmpeg cannot be found")


def main():
    param = None

    parsed_argv = []
    for index, argv in enumerate(sys.argv):
        if index == 0:
            continue
        if argv == "-a":
            param = YT_Parameter.AUDIO
        elif argv == "-v":
            param = YT_Parameter.VIDEO
        elif argv == "-b":
            param = YT_Parameter.BOTH
        else:
            parsed_argv.append(argv)

    if param is None:
        print("Either -a(Audio), -v(Video) or -b(Both) is required...")
    else:
        for argv in parsed_argv:
            try:
                yt = YT(argv, param)
                yt.download(callback)
            except KeyboardInterrupt:
                print("Ctrl-C detected, quitting")
                pass


if __name__ == "__main__":
    main()
