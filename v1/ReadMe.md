# RKExport
**Утилита для экспорта данных из *RKeeper7*.** 

### Конфигурационный файл.
Имя конфигурационного файла может быть передано в виде параметра при запуске программы. Если параметр отсутствует конфигурационный файл должен
присутствовать в каталоге с программой. Имя файла при этом должно совпадать с именем программы с расшиирением *.json*
Если конфигурационный файл не найден, программа не запустится, будет выдано соответствующее сообщение об ошибке.
**Параметры конфигурационного файла:**
- *logTo* Строка, управляет выводом лога, возможные значения: stdout, file.
- *logFile* Строка, имя файла лога при выводе в файлa.
- *logLevel* Строка, устанавливает уровень логирования. Возможные значения: debug|info|error
- *msCon* Строка, соединение с базой MSSQL RKeeper7. Формат: "sqlserver://USER:PASSWORD@SERVER:1433/INSTANCE?database=DATABASE"
- *restaurants* Массив строк, наименование ресторанов для эскпорта. Если в массиве есть хоть один элемент, будет задан фильтр. Если параметр конфигурации опущен, или массив пустой, будут экспортированы все рестораны.
- *cashGroups* Массив строк. Фильтрация по кассовым серверам. Если параметр не задан или пустой - без фильтрации.
- *webServer* Структура слдующего состава:
    - *credential* Строка в формате ИмяПользователя:Пароль
    - *host* Строка в формат: IP:PORT
    - *idleTimeout* Целое число, интервал в мс в течении которого сервер не закрывает idle соединение
    - *readTimeout* Целое число, интервал в мс для чтения запроса
    - *writeTimeout* Целое число, интервал в мс для отправки ответа на запрос
    - *handlerTimeout* Целое число, интервал в мс для функции обработки запроса

### Файл sql запроса.
Шаблон файла запроса должен присутствовать в каталоге с программой под именем msQuery.sql. Файл имеет следующие параметры:
- {{COND}} Данная строка будет заменена на условие запроса с фильтром по периоду, выбранным рестаранам, кассовым серверам.
- {{FROM}} Параметр from запроса http - номер записи, с которой начать экспорт.
- {{COUNT}} Параметр count запроса http - количество записей.

### Формат http запроса.
Утилита принимает запросы по адресу /query.
Обязательным условием является запрос с типом GET и наличием заголовка "Authorization" со значением Basic base64(USER:PASSWORD). Имя пользователя и пароль будут сопоставлены с настройками на сервере.
При отсутствии заголовка или неверных данных сервер ответит кодом Unauthorized 401.
Запрос принимает следующие параметры:
- date_from Дата в формате 2006-01-02T15:04:05.999 Обязательный парамтр.
- date_to Дата в формате 2006-01-02T15:04:05.999 Обязательный парамтр.
- from Целое число, номер записи для экспорта, начиная с 0. Не обазятельный, по-умолчанию 0.
- count Целое число, количество записей для экспорта. Не обязательный, по умолчанию 100.
Параметры передаются в строке url, /query?count=10&from=5&date_from=2024-05-08T00:00:00&date_to=2024-05-08T23:59:59
**Пример запроса с помощью curl**
```
#Получим base64(user:123456)
echo -n 'user:123456' | base64
curl --verbose 'http://localhost:59000/query?date_from=2024-05-01T00:00:00&date_to=2024-05-01T23:59:59' --header 'Authorization: Basic dXNlcjoxMjM0NTY='
```
### Сборка.
В Windows:
```
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui" -o rkexport.exe
```
В Linux:
```
go build -ldflags "-s -w" -o krexport .
```
### Формат ответа
При успешном выполнении запроса тело ответа будет содержать массив струтур вида:
- restaurantId    int       // • ID ресторана 
- cashGroupId     int       // • ID кассы 
- visitId         int       // • ID посещения 
- checkOpen       time.Time // • Дата/время открытия/закрытия заказа
- checkClose      time.Time // • Дата/время открытия/закрытия заказа
- visitStartTime  time.Time // • Дата/время формирования пречека
- orderNum        string    // • Номер заказа
- fiscDocNum      string    // • Фискализация
- orderSum        float64   // • Сумма заказа до применения скидок
- paySum          float64   // • Фактическая сумма заказа, оплаченная пользователем (после применения скидок)
- itemCount       int       // • Кол-во позиций в чеке
- payType         string    // • Способ оплаты (нал/безнал/иное)
- discountSum     float64   // • Сумма использованных бонусов/скидок и комментарий по ним
- discountComment string    // • Признак удаления заказа или его сторнирования
