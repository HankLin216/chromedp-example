# About

You can set the date and time for automatic booking of badminton courts at Tatong Sports Center. You can also specify the time when the automatic reservation program is activated.

# Download
https://github.com/HankLin216/chromedp-example/releases  
  
![Dowload path picture](https://github.com/HankLin216/chromedp-example/blob/master/datong-sportcenter-badminton/images/downloadPath.png?raw=true)


# Usage

To run the program in cmd or PowerShell, use the following command (Only necessary parameters are displayed):

``` cmd
.\datong_sportcenter_badminton.exe -n [身分證字號] -p [密碼] -h [要預約場地的時間]
```
### ☀️Example
- 只使用必要參數:  
效果是"立刻"自動預約定位7天後的早上8點和傍晚18點
``` cmd
.\datong_sportcenter_badminton.exe -n "A123456789" -p "password" -h 8,18
```

- 指定日期:
效果是"立刻"自動預約定位2月16日的早上8點和傍晚18點
``` cmd
.\datong_sportcenter_badminton.exe -n "A123456789" -p "password" -h 8,18 -d "02-16"
```

- 不要真的預約，試跑想看結果:  
效果是"立刻"自動預約定位2月16日的早上8點和傍晚18點，但會在最後確定的視窗停下來。  

``` cmd
.\datong_sportcenter_badminton.exe -n "A123456789" -p "password" -h 8,18 -d "02-16" --dry=true
```

- 想要在指定的時間執行自動預約:
效果是會在"02.14的晚上10點45分30秒"自動預約定位2月16日的早上8點和傍晚18點。  
``` cmd
.\datong_sportcenter_badminton.exe -n "A123456789" -p "password" -h 8,18 -d "02-16" -c "30 45 10 14 2 ?"
```
- 想要每周五早上執行自動預約下周六的時段:
效果是會在"每周五凌晨"自動預約定位7天後(下周五)的早上8點和傍晚18點。  
``` cmd
.\datong_sportcenter_badminton.exe -n "A123456789" -p "password" -h 8,18 -c "0 0 0 ? * 5"
```

### ⚡ All Program Parameters
- -c string
你想要啟動自動預約的時間與日期，格式為cron表達示。  
都不輸入表示立刻啟動。  
<span style="color:#FFECA1">超重要: 只支援6個欄位，分別表示，秒、分、時、日、月、星期。</span>  
例如:  
  -  0 40 13 23 4 ? 表示 0秒、40分、13點、23日、4月、忽略。
  -  0 0 0 ? * 5 表示 0秒、0分、0點、忽略、每月、每個星期五。
  -  0 0 2 ? 3 0 表示 0秒、0分、2點、忽略、3月、每個星期日。  

想要客製化更多時段，請查詢cron expression (記得不要輸入年喔)  
參考網頁 https://cron.ciding.cc/

- -d string  
哪一天呢，接受格式是<span style="color:#FFECA1">MM-DD</span>，且為一個星期內。例如你想要預約2月16號，請輸入02-16。  
沒輸入的話就當作是要抓<span style="color:#FFECA1">可預約的最後一天</span>，例如: 今天是星期六直接預約下星期日。

- -dry  
要不要真的訂位，想試跑看看的話就輸入true，這個參數要加=，例如<span style="color:#FFECA1">-dry=true</span> (default false) 

- -h string  
你想要預約的時段，每個時段長度預約為一小時，只<span style="color:#FFECA1">接受格式為24小時制</span>，範圍為6-21。  
可以支援多個時段，<span style="color:#FFECA1">請用，號隔開</span>。例如你想要14到16，請輸入14,15。

- -lrt int  
<span style="color:#FFECA1">進階設定</span>，登錄的重試次數 (default 5)

- -ltp int  
<span style="color:#FFECA1">進階設定</span>，登錄的重試等待週期(毫秒)，網頁網速夠快可以調低試試看 (default 300)

- -n string  
身分證字號

- -p string  
噓....秘密

- -rrt int  
<span style="color:#FFECA1">進階設定</span>，點擊預約按鈕的重試次數，這就不建議亂調了 (default 5)

- -rtp int  
<span style="color:#FFECA1">進階設定</span>，等待預約警告視窗出現的週期(毫秒)，這就不建議亂調了 (default 150)
