::
:: Licensed under the Apache License, Version 2.0 (the "License");
:: you may not use this file except in compliance with the License.
:: You may obtain a copy of the License at
::
::     http://www.apache.org/licenses/LICENSE-2.0
::
:: Unless required by applicable law or agreed to in writing, software
:: distributed under the License is distributed on an "AS IS" BASIS,
:: WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
:: See the License for the specific language governing permissions and
:: limitations under the License.
::

SETLOCAL EnableDelayedExpansion

SET URLFILE=libevent-%LIBEVENT_VERSION%-stable.tar.gz
SET URL=https://github.com/libevent/libevent/releases/download/release-%LIBEVENT_VERSION%-stable/%URLFILE%

CD %WIN3P%                                       || EXIT /B
appveyor DownloadFile %URL%                      || EXIT /B
7z x %URLFILE% -so | 7z x -si -ttar > nul        || EXIT /B
CD "libevent-%LIBEVENT_VERSION%-stable"          || EXIT /B
nmake -f Makefile.nmake                          || EXIT /B
mkdir lib                                        || EXIT /B
move *.lib lib\                                  || EXIT /B
move WIN32-Code\event2\* include\event2\         || EXIT /B
move *.h include\                                || EXIT /B

ENDLOCAL
