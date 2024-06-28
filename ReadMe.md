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
- *apiUrl* URL для вызова API, содержит параметры {{scID}} {{saleLocationID}}, которые будут заменены на конфигурационные данные.
- *apiCmdGetPeriod* подкаталог API для получения периода, например last_sale_date/
- *apiCmdPutData* подкаталог API для отправки данных, например create_order/
- *apiKey* Строка с ключом для проверки, отправляется в запросе как заголовок api-token.
- *activationTime* Время в формате 00:00
- *saleLocationID* Строка с идентификатором.
- *scID* Строка с идентификатором.

### Файл sql запроса.
Шаблон файла запроса должен присутствовать в каталоге с программой под именем msQuery.sql. Файл имеет следующие параметры:
- {{COND}} Данная строка будет заменена на условие запроса с фильтром по периоду, выбранным рестаранам, кассовым серверам.
- {{FROM}} Параметр from запроса http - номер записи, с которой начать экспорт.
- {{COUNT}} Параметр count запроса http - количество записей.

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
