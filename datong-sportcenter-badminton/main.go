package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	c "github.com/chromedp/chromedp"
	"github.com/robfig/cron/v3"
)

var (
	// input
	acc                string
	pwd                string
	hoursToReserveStr  string
	dateStr            string
	cronExpr           string
	lauch              bool
	dryRun             bool
	loginRetryTimes    int
	loginRetryPeriod   int
	reserveRetryTimes  int
	reserveRetryPeriod int

	// parse
	hoursToReserve []int
	reserveMonth   int
	reserveDay     int
)

type LogType int

const (
	DEBUG LogType = iota
	WARNING
	ERROR
	SUCCESS
	SYSTEM
)

const (
	loginUrl       = "https://bwd.xuanen.com.tw/wd02.aspx?module=login_page&files=login"
	portalUrl      = "https://bwd.xuanen.com.tw/wd02.aspx?Module=ind&files=ind"
	reserveUrlBase = "https://bwd.xuanen.com.tw/wd02.aspx?module=net_booking&files=booking_place&StepFlag=2&PT=1&D="
	orderUrl       = "https://bwd.xuanen.com.tw/wd02.aspx?module=member&files=orderx_mt"
)

func init() {
	flag.StringVar(&acc, "n", "", "身分證字號")
	flag.StringVar(&pwd, "p", "", "噓....秘密")
	flag.StringVar(&dateStr, "d", "", "哪一天呢，接受格式是MM-DD，且為一個星期內。例如你想要預約2月16號，請輸入02-16。\n沒輸入的話就當作是要抓可預約的最後一天，例如: 今天是星期六直接預約下星期日")
	flag.StringVar(&hoursToReserveStr, "h", "", "你想要預約的時段，每個時段長度預約為一小時，只接受格式為24小時制，範圍為6-21。\n可以支援多個時段，請用，號隔開。\n例如你想要14到16，請輸入14,15")
	cronExplain := "你想要啟動自動預約的時間與日期，格式為cron表達示。\n"
	cronExplain += "都不輸入表示立刻啟動。\n"
	cronExplain += "*超重要:* 只支援6個欄位，分別表示，秒、分、時、日、月、星期。\n"
	cronExplain += "例如:\n"
	cronExplain += "     0 40 13 23 4 ? 表示 0秒、40分、13點、23日、4月、忽略。\n"
	cronExplain += "     0 0 0 ? * 5 表示 0秒、0分、0點、忽略、每月、每個星期五。\n"
	cronExplain += "     0 0 2 ? 3 0 表示 0秒、0分、2點、忽略、3月、每個星期日。\n"
	cronExplain += "想要客製化更多時段，請查詢cron expression (記得不要輸入年)\n"
	flag.StringVar(&cronExpr, "c", "", cronExplain)
	flag.BoolVar(&lauch, "l", true, "要不要開瀏覽器，不想開就輸入false，這個參數要加=，例如-l=false")
	flag.BoolVar(&dryRun, "dry", false, "要不要真的訂位，想試跑看看的話就輸入true，這個參數要加=，例如-dry=true")
	flag.IntVar(&loginRetryTimes, "lrt", 5, "進階設定，登錄的重試次數")
	flag.IntVar(&loginRetryPeriod, "ltp", 300, "進階設定，登錄的重試等待週期(毫秒)，網頁網速夠快可以調低試試看")
	flag.IntVar(&reserveRetryTimes, "rrt", 5, "進階設定，點擊預約按鈕的重試次數，這就不建議亂調了")
	flag.IntVar(&reserveRetryPeriod, "rtp", 150, "進階設定，等待預約警告視窗出現的週期(毫秒)，這就不建議亂調了")
	flag.Usage = usage
}

func usage() {
	fmt.Fprint(os.Stderr, "服用方法: \n")
	fmt.Fprint(os.Stderr, ".\\datong_sportcenter_badminton.exe -n [身分證字號] -p [密碼] -h [要預約場地的時間] -c [要執行自動預約的時間與日期]\n")
	flag.PrintDefaults()
}

func parseHoursToReserve() (bool, error) {

	if hoursToReserveStr == "" {
		return false, fmt.Errorf("時間勒?")
	}

	hours := strings.Split(hoursToReserveStr, ",")
	for _, hour := range hours {
		hour := strings.Trim(hour, " ")
		hourInt, err := strconv.Atoi(hour)
		if err != nil {
			return false, fmt.Errorf("看不懂的預定時段..., 解析失敗的輸入為: %s, err: %s", hour, err.Error())
		}

		if hourInt < 0 {
			return false, fmt.Errorf("你這個%s....負數是?", hour)
		}

		if hourInt < 6 || hourInt > 21 {

			cerr := fmt.Errorf("開館時間是6-22喔，%s超過可預約的時間", hour)
			if hourInt == 22 {
				cerr = fmt.Errorf("開館時間是6-22喔，%s已經閉館了", hour)
			}

			return false, cerr
		}

		hoursToReserve = append(hoursToReserve, hourInt)
	}

	return true, nil
}

func isValidDate(month int, day int) bool {
	// get next month
	t := time.Date(time.Now().Year(), time.Month(month)+1, 1, 0, 0, 0, 0, time.UTC)

	// get the latest day of this month
	lastDay := t.AddDate(0, 0, -1).Day()

	// check the date is between this month
	return day >= 1 && day <= lastDay
}

func parseDate() (bool, error) {

	if dateStr != "" {
		ms := ""
		ds := ""
		r := regexp.MustCompile(`^(?P<month>\d{1,2})-(?P<day>\d{2})$`)
		gns := r.SubexpNames()
		for _, m := range r.FindAllStringSubmatch(dateStr, -1) {
			for i, g := range m {
				n := gns[i]
				if n == "month" {
					ms = g
				}

				if n == "day" {
					ds = g
				}
			}
		}

		if ms == "" || ds == "" {
			return false, fmt.Errorf("不符合格式的日期..., 解析失敗的輸入為: %s", dateStr)
		}

		// convert to int
		_reserveMonth, err := strconv.Atoi(ms)
		if err != nil {
			return false, fmt.Errorf("解析不了月份..., 解析失敗的月份為: %s", ms)
		}

		reserveMonth = _reserveMonth

		_reserveDay, err := strconv.Atoi(ds)
		if err != nil {
			return false, fmt.Errorf("解析不了日期..., 解析失敗的日期為: %s", ds)
		}

		reserveDay = _reserveDay

		// check the date is valid
		if !isValidDate(reserveMonth, reserveDay) {
			return false, fmt.Errorf("這個日期是不是有點太超過了..., 輸入月份日期為: %s", dateStr)
		}

		// check date within one week
		// convert to date
		i := time.Date(time.Now().Year(), time.Month(reserveMonth), reserveDay, 0, 0, 0, 0, time.UTC)
		nt := time.Now()

		// start date
		sd := nt.AddDate(0, 0, 1)

		// end date
		ed := nt.AddDate(0, 0, 8)
		if !i.After(sd) || !i.Before(ed) {
			return false, fmt.Errorf("只能輸入%d-%d到%d-%d之間喔, 但你輸入月份日期為: %s", int(sd.Month()), sd.Day(), int(ed.Add(-time.Hour*24).Month()), ed.Add(-time.Hour*24).Day(), dateStr)
		}
	} else {
		// defaults to the last day of the next week
		d := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
		d = d.AddDate(0, 0, 7)
		reserveMonth = int(d.Month())
		reserveDay = d.Day()
	}

	return true, nil
}

func joinIntSlice(s []int) string {
	if len(s) == 0 {
		return ""
	}

	ss := make([]string, len(s))
	for i, v := range s {
		ss[i] = strconv.Itoa(v)
	}

	return strings.Join(ss, ",")
}

func checkArgsAndShow() bool {
	// check id and password
	if acc == "" || pwd == "" {
		printf("帳號密碼勒?\n", ERROR)
		return false
	}

	// check date
	if ok, err := parseDate(); !ok {
		printf(fmt.Sprintf("日期解析失敗: %s\n", err.Error()), ERROR)
		return false
	}

	// check hours to reserve
	if ok, err := parseHoursToReserve(); !ok {
		printf(fmt.Sprintf("時間解析失敗: %s\n", err.Error()), ERROR)
		return false
	}

	// check cron expression is valid
	cronExprString := ""
	if cronExpr != "" {
		cronExprString = cronExpr
		parser := cron.NewParser(
			cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
		)
		_, err := parser.Parse(cronExpr)
		if err != nil {
			printf(fmt.Sprintf("Cron expression解析失敗: %s\n", err.Error()), ERROR)
			return false
		}
	} else {
		cronExprString = "立刻，馬上，刻不容緩，不管甚麼表達示，我就是要現在"
	}

	// show the args
	printf("===============隨便分隔線===============\n", SYSTEM)
	printf("翻譯您輸入的值:\n", SYSTEM)
	printf(fmt.Sprintf("  身份證字號: %s\n", acc), SYSTEM)
	printf("  密碼: 你猜\n", SYSTEM)
	printf(fmt.Sprintf("  想預約的日期: %d-%d\n", reserveMonth, reserveDay), SYSTEM)
	printf(fmt.Sprintf("  想預約的時段: %s\n", joinIntSlice(hoursToReserve)), SYSTEM)
	printf(fmt.Sprintf("  想執行自動預約的cron表達示: %s\n", cronExprString), SYSTEM)
	printf(fmt.Sprintf("  想看瀏覽器跑來跑去: %t\n", lauch), SYSTEM)
	printf(fmt.Sprintf("  假跑?: %t\n", dryRun), SYSTEM)
	printf(fmt.Sprintf("  *登錄的重試次數: %d\n", loginRetryTimes), SYSTEM)
	printf(fmt.Sprintf("  *登錄的重試等待週期(毫秒): %d\n", loginRetryPeriod), SYSTEM)
	printf(fmt.Sprintf("  *點擊預約按鈕的重試次數: %d\n", reserveRetryTimes), SYSTEM)
	printf(fmt.Sprintf("  *等待預約警告視窗出現的週期(毫秒): %d\n", reserveRetryPeriod), SYSTEM)
	printf("===============隨便分隔線===============\n", SYSTEM)

	return true
}

func registerAlertDialogListen(ctx context.Context, id int, result chan<- string) error {
	return c.Run(ctx,
		c.ActionFunc(func(ctx context.Context) error {
			c.ListenTarget(ctx, func(ev interface{}) {
				// page loaded
				if _, ok := ev.(*page.EventJavascriptDialogOpening); ok {
					printf(fmt.Sprintf("[%d][System]Alert視窗內容:\n%s\n", id, ev.(*page.EventJavascriptDialogOpening).Message), WARNING) // holds msg!

					isEnsureAlertDialog := strings.Contains(ev.(*page.EventJavascriptDialogOpening).Message, "您是否確定預約")
					if isEnsureAlertDialog {

						go func() {
							result <- "ensure alert dialog shown"
						}()

						if dryRun {
							printf(fmt.Sprintf("[%d][System]就按到這邊，剩下交給你了\n", id), SYSTEM)
							return
						}
					}

					// 確認Alert視窗
					go func() {
						if err := c.Run(ctx,
							page.HandleJavaScriptDialog(true),
						); err != nil {
							log.Fatal(err)
						}
					}()
				}
			})
			return nil
		}),
	)
}

func login(ctx context.Context, id int, retry int, periodInMillisecond int) error {
	url := ""
	for i := 0; i < retry; i++ {
		printf(fmt.Sprintf("[%d][Debug] %d'th try login\n", id, i+1), DEBUG)

		// wait a moment
		time.Sleep(time.Duration(periodInMillisecond) * time.Millisecond)
		// click login button
		printf(fmt.Sprintf("[%d][Debug] click login button\n", id), DEBUG)
		err := c.Run(ctx,
			c.ActionFunc(func(ctx context.Context) error {
				var nodes []*cdp.Node
				if err := c.Nodes(`input[name="login_but"]`, &nodes, c.AtLeast(0)).Do(ctx); err != nil {
					return err
				}

				if len(nodes) == 0 {
					return nil
				}

				return c.MouseClickNode(nodes[0]).Do(ctx)
			}),
		)

		if err != nil {
			return err
		}

		// 點完等一下
		time.Sleep(time.Duration(periodInMillisecond) * time.Millisecond)

		// 檢查網頁是否跳轉
		printf(fmt.Sprintf("[%d][Debug] check url\n", id), DEBUG)
		err = c.Run(ctx,
			c.Location(&url),
		)

		if err != nil {
			return err
		}

		if url == portalUrl {
			printf(fmt.Sprintf("[%d][Debug] url correct, already redirect to new web page\n", id), DEBUG)
			break
		}

		// 檢查完等一下
		time.Sleep(time.Duration(periodInMillisecond) * time.Millisecond)

		// 看看是不是有失敗的dialog
		printf(fmt.Sprintf("[%d][Debug] click the verification failed button\n", id), DEBUG)
		err = c.Run(ctx,
			c.ActionFunc(func(ctx context.Context) error {

				var nodes []*cdp.Node
				if err := c.Nodes("/html/body/div[2]/div/div[3]/button[1]", &nodes, c.AtLeast(0)).Do(ctx); err != nil {
					return err
				}

				if len(nodes) == 0 {
					return nil
				}

				return c.MouseClickNode(nodes[0]).Do(ctx)
			}),
		)

		if err != nil {
			return err
		}
	}

	if url != portalUrl {
		return fmt.Errorf("登錄失敗，已嘗試%d次了", retry)
	}

	return nil
}

func getCellIndex(v int) (start int, end int) {
	divisor := 6
	remainder := v % divisor

	start = remainder*13 + 2
	end = start + 12
	return
}

func ensure(ctx context.Context, id int, reserveTime int, result <-chan string, retry int, periodInMillisecond int) error {

	// 取得時段在表格的位置
	s, e := getCellIndex(reserveTime)
	for i := 0; i < retry; i++ {
		printf(fmt.Sprintf("[%d][Debug] %d'th try click reserve\n", id, i+1), DEBUG)
		printf(fmt.Sprintf("[%d][Debug] click reserve image\n", id), DEBUG)

		err := c.Run(ctx,
			c.ActionFunc(func(ctx context.Context) error {
				expr := fmt.Sprintf("//*[@id=\"ContentPlaceHolder1_Step2_data\"]/table/tbody/tr[position()=%d]/td[4]/img[@name=\"PlaceBtn\"] | //*[@id=\"ContentPlaceHolder1_Step2_data\"]/table/tbody/tr[position()>=%d and position()<=%d]/td[3]/img[@name=\"PlaceBtn\"]", s, s+1, e)
				var nodes []*cdp.Node
				if err := c.Nodes(expr, &nodes, c.AtLeast(0)).Do(ctx); err != nil {
					return c.Error("搜尋時段在表格的位置時發生錯誤!")
				}

				if len(nodes) == 0 {
					return c.Error("都被預約完拉~")
				}

				// 有的話直接預約第一個0
				return c.MouseClickNode(nodes[0]).Do(ctx)
			}),
		)

		if err != nil {
			return err
		}

		// 等待alert視窗
		select {
		case res := <-result:
			printf(fmt.Sprintf("[%d][Debug] %s\n", id, res), DEBUG)
			return nil
		case <-time.After(time.Duration(periodInMillisecond) * time.Millisecond):
			printf(fmt.Sprintf("[%d][Debug] already over %d millisecond\n", id, periodInMillisecond), DEBUG)
			continue
		}
	}

	return fmt.Errorf("預約失敗，已嘗試%d次了", retry)
}

func reserve(id int, lauch bool, dryRun bool, reserveTime int, reserveMonth int, reserveDay int) error {

	// create chan
	ch := make(chan string)
	defer close(ch)

	// options
	opts := append(c.DefaultExecAllocatorOptions[:],
		c.Flag("headless", !lauch),
	)

	allocCtx, cancel := c.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// set up a custom logger
	taskCtx, cancel := c.NewContext(allocCtx, c.WithLogf(log.Printf))
	defer cancel()

	// ensure that the browser process is started
	if err := c.Run(taskCtx); err != nil {
		return fmt.Errorf(fmt.Sprintf("[Error][1]: %s", err.Error()))
	}

	// region register listener for alert dialog
	printf(fmt.Sprintf("[%d][System]開始監聽網頁Alert視窗，但我只會一直點確定...\n", id), SYSTEM)
	if dryRun {
		printf(fmt.Sprintf("[%d][System]但最後關頭我不會點確定的，放心\n", id), SYSTEM)
	}

	err := registerAlertDialogListen(taskCtx, id, ch)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("[Error][2]: %s", err.Error()))
	}

	// visit
	webTitle := ""
	err = c.Run(taskCtx,
		c.Navigate(loginUrl),
		c.Title(&webTitle),
	)

	if err != nil {
		return fmt.Errorf(fmt.Sprintf("[Error][3]: %s", err.Error()))
	}

	if webTitle == "" {
		return fmt.Errorf("[System]????登到哪去了，找你的bro看一下問題")
	}

	printf(fmt.Sprintf("[%d][System]開始訪問: %s \n", id, webTitle), SYSTEM)

	// 輸入帳號密碼
	printf(fmt.Sprintf("[%d][System]輸入帳號密碼囉!\n", id), SYSTEM)
	err = c.Run(taskCtx,
		// 點擊彈出視窗
		c.WaitReady(`button.swal2-confirm.swal2-styled`, c.ByQuery),
		c.Click(`button.swal2-confirm.swal2-styled`, c.NodeVisible),
		// 輸入帳號密碼
		c.SendKeys(`input[name="ctl00$ContentPlaceHolder1$loginid"]`, acc),
		c.SendKeys(`input[name="loginpw"]`, pwd),
	)

	if err != nil {
		return fmt.Errorf(fmt.Sprintf("[Error][4]: %s", err.Error()))
	}

	printf(fmt.Sprintf("[%d][System]登錄登錄登錄!\n", id), SYSTEM)
	printf(fmt.Sprintf("[%d][Debug] retry times: %d, retry period(ms): %d!\n", id, loginRetryTimes, loginRetryPeriod), DEBUG)
	err = login(taskCtx, id, loginRetryTimes, loginRetryPeriod)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("[Error][5]: %s", err.Error()))
	}

	// 直接跳到預約日期
	timeValue := "1" // 下午
	if reserveTime >= 12 && reserveTime <= 17 {
		timeValue = "2" // 下午
	} else if reserveTime > 17 {
		timeValue = "3" // 晚上
	}

	year := time.Now().Year()
	yearStr := fmt.Sprintf("%d", year)
	monthStr := fmt.Sprintf("%02d", reserveMonth)
	dayStr := fmt.Sprintf("%02d", reserveDay)
	url := fmt.Sprintf(`%s%s/%s/%s&D2=%s`, reserveUrlBase, yearStr, monthStr, dayStr, timeValue)
	err = c.Run(taskCtx,
		c.Navigate(url),
		// 等一下表格出現
		c.WaitVisible(`#ContentPlaceHolder1_Step2_data`, c.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("[Error][10]: %s", err.Error())
	}

	// 選擇可預定場地
	err = ensure(taskCtx, id, reserveTime, ch, reserveRetryTimes, reserveRetryPeriod)
	if err != nil {
		return fmt.Errorf("[Error][11]: %s", err.Error())
	}

	// 確認是否預約成功
	if !dryRun {
		info := ""
		err = c.Run(taskCtx,
			c.WaitVisible("//*[@id=\"ContentPlaceHolder1_Step3Info_lab\"]", c.BySearch),
			c.Text("//*[@id=\"ContentPlaceHolder1_Step3Info_lab\"]/span[2]", &info, c.BySearch),
		)

		if err != nil {
			return fmt.Errorf("[Error][12]: %s", err.Error())
		}

		if strings.Contains(info, "您今日已預約超過可預約場地2場次(2小時)") {
			return fmt.Errorf("[Error]您今日已預約超過可預約場地2場次(2小時)")
		}

		if strings.Contains(info, "網路繁忙中，請您稍後重新執行預約作業") {
			return fmt.Errorf("[Error]網頁顯示:網路繁忙中, 可能被搶走了")
		}

	}

	return nil
}

func printf(log string, t LogType) {

	ts := time.Now()
	tsf := ts.Format("2006-01-02 15:04:05")

	colorS := "\033[1;34m" // blue
	colorE := "\033[0m"

	switch t {
	case SUCCESS: // green
		colorS = "\033[1;32m"
		colorE = "\033[0m"
	case DEBUG: // yellow
		colorS = "\033[1;33m"
		colorE = "\033[0m"
	case ERROR: // red
		colorS = "\033[1;31m"
		colorE = "\033[0m"
	case WARNING: // gray
		colorS = "\033[1;30m"
		colorE = "\033[0m"
	}

	fmt.Printf("%s[%s]%s%s", colorS, tsf, log, colorE)
}

func start() {
	printf("[System]時間到拉，我要開始去臺北大同運動中心網站搶羽球場了喔!\n", SYSTEM)

	var wg sync.WaitGroup
	for i, v := range hoursToReserve {
		wg.Add(1)
		go func(id int, t int) {
			defer wg.Done()

			printf(fmt.Sprintf("[%d]趕快來搶%d月%d日的%d點的羽球場\n", id, reserveMonth, reserveDay, t), SYSTEM)
			err := reserve(id, lauch, dryRun, t, reserveMonth, reserveDay)
			if err != nil {
				printf(fmt.Sprintf("[%d]%s\n", id, err.Error()), ERROR)
				printf(fmt.Sprintf("[%d][ಥ_ಥ]搶%d月%d日的%d點的羽球場失敗了!\n", id, reserveMonth, reserveDay, t), ERROR)
			} else {
				if !dryRun {
					printf(fmt.Sprintf("[%d][(≧∀≦)ゞ]搶%d月%d日的%d點的羽球場成功拉!\n", id, reserveMonth, reserveDay, t), SUCCESS)
				} else {
					printf(fmt.Sprintf("[%d][(≖ᴗ≖๑)]完成搶%d月%d日的%d點的羽球場囉!\n", id, reserveMonth, reserveDay, t), SUCCESS)
				}

			}
		}(i, v)
	}

	wg.Wait()
	printf(fmt.Sprintf("[System]訂單在這，%s\n", orderUrl), SYSTEM)
	printf("[System]Press 'Enter' to continue...\n", SYSTEM)
}

func main() {
	flag.Parse()
	if !checkArgsAndShow() {
		flag.Usage()
		return
	}

	if cronExpr == "" {
		start()
	} else {
		printf("你有設定時間，到了就會執行了，有點耐心\n", WARNING)

		c := cron.New(cron.WithSeconds(), cron.WithParser(cron.NewParser(
			cron.SecondOptional|cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow,
		)))
		c.AddFunc(cronExpr, start)
		c.Start()
	}

	bufio.NewReader(os.Stdin).ReadBytes('\n')
}
